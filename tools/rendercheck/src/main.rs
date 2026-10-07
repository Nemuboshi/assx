use libloading::Library;
use regex::Regex;
use std::collections::BTreeSet;
use std::env;
use std::ffi::CString;
use std::fs;
use std::os::raw::{c_char, c_int};
use std::path::{Path, PathBuf};
use std::ptr;

#[repr(C)]
struct AssLibrary {
    _private: [u8; 0],
}

#[repr(C)]
struct AssRenderer {
    _private: [u8; 0],
}

#[repr(C)]
struct AssTrack {
    _private: [u8; 0],
}

#[repr(C)]
struct AssImage {
    w: c_int,
    h: c_int,
    stride: c_int,
    bitmap: *mut u8,
    color: u32,
    dst_x: c_int,
    dst_y: c_int,
    next: *mut AssImage,
    image_type: c_int,
}

type AssLibraryInit = unsafe extern "C" fn() -> *mut AssLibrary;
type AssLibraryDone = unsafe extern "C" fn(*mut AssLibrary);
type AssAddFont = unsafe extern "C" fn(*mut AssLibrary, *const c_char, *const c_char, c_int);
type AssSetExtractFonts = unsafe extern "C" fn(*mut AssLibrary, c_int);
type AssRendererInit = unsafe extern "C" fn(*mut AssLibrary) -> *mut AssRenderer;
type AssRendererDone = unsafe extern "C" fn(*mut AssRenderer);
type AssSetStorageSize = unsafe extern "C" fn(*mut AssRenderer, c_int, c_int);
type AssSetFrameSize = unsafe extern "C" fn(*mut AssRenderer, c_int, c_int);
type AssSetFonts = unsafe extern "C" fn(
    *mut AssRenderer,
    *const c_char,
    *const c_char,
    c_int,
    *const c_char,
    c_int,
);
type AssReadFile =
    unsafe extern "C" fn(*mut AssLibrary, *const c_char, *const c_char) -> *mut AssTrack;
type AssFreeTrack = unsafe extern "C" fn(*mut AssTrack);
type AssRenderFrame =
    unsafe extern "C" fn(*mut AssRenderer, *mut AssTrack, i64, *mut c_int) -> *mut AssImage;

struct Api {
    _library: Library,
    library_init: AssLibraryInit,
    library_done: AssLibraryDone,
    add_font: AssAddFont,
    set_extract_fonts: AssSetExtractFonts,
    renderer_init: AssRendererInit,
    renderer_done: AssRendererDone,
    set_storage_size: AssSetStorageSize,
    set_frame_size: AssSetFrameSize,
    set_fonts: AssSetFonts,
    read_file: AssReadFile,
    free_track: AssFreeTrack,
    render_frame: AssRenderFrame,
}

impl Api {
    unsafe fn load(path: &Path) -> Result<Self, String> {
        let library = unsafe { Library::new(path) }
            .map_err(|e| format!("failed to load libass from {}: {e}", path.display()))?;

        unsafe fn sym<T: Copy>(lib: &Library, name: &[u8]) -> Result<T, String> {
            unsafe { lib.get::<T>(name) }.map(|s| *s).map_err(|e| {
                format!(
                    "missing libass symbol {}: {e}",
                    String::from_utf8_lossy(name)
                )
            })
        }

        Ok(Self {
            library_init: unsafe { sym(&library, b"ass_library_init\0")? },
            library_done: unsafe { sym(&library, b"ass_library_done\0")? },
            add_font: unsafe { sym(&library, b"ass_add_font\0")? },
            set_extract_fonts: unsafe { sym(&library, b"ass_set_extract_fonts\0")? },
            renderer_init: unsafe { sym(&library, b"ass_renderer_init\0")? },
            renderer_done: unsafe { sym(&library, b"ass_renderer_done\0")? },
            set_storage_size: unsafe { sym(&library, b"ass_set_storage_size\0")? },
            set_frame_size: unsafe { sym(&library, b"ass_set_frame_size\0")? },
            set_fonts: unsafe { sym(&library, b"ass_set_fonts\0")? },
            read_file: unsafe { sym(&library, b"ass_read_file\0")? },
            free_track: unsafe { sym(&library, b"ass_free_track\0")? },
            render_frame: unsafe { sym(&library, b"ass_render_frame\0")? },
            _library: library,
        })
    }
}

struct RenderContext<'a> {
    api: &'a Api,
    library: *mut AssLibrary,
    renderer: *mut AssRenderer,
    track: *mut AssTrack,
    width: usize,
    height: usize,
    _font_names: Vec<CString>,
    _font_data: Vec<Vec<u8>>,
    _default_font: CString,
    _ass_path: CString,
}

impl<'a> RenderContext<'a> {
    unsafe fn new(
        api: &'a Api,
        ass_path: &Path,
        fonts_dir: &Path,
        width: usize,
        height: usize,
    ) -> Result<Self, String> {
        let font_files = collect_font_files(fonts_dir)?;
        let default_font_c = path_cstring(&font_files[0])?;
        let ass_path_c = path_cstring(ass_path)?;

        let library = unsafe { (api.library_init)() };
        if library.is_null() {
            return Err("ass_library_init failed".into());
        }

        unsafe { (api.set_extract_fonts)(library, 1) };

        let mut font_names = Vec::with_capacity(font_files.len());
        let mut font_data = Vec::with_capacity(font_files.len());
        for path in &font_files {
            let name = CString::new(
                path.file_name()
                    .and_then(|name| name.to_str())
                    .ok_or_else(|| format!("font has no UTF-8 file name: {}", path.display()))?,
            )
            .map_err(|_| format!("font name contains a NUL byte: {}", path.display()))?;
            let data =
                fs::read(path).map_err(|e| format!("cannot read font {}: {e}", path.display()))?;
            unsafe {
                (api.add_font)(
                    library,
                    name.as_ptr(),
                    data.as_ptr() as *const c_char,
                    data.len() as c_int,
                );
            }
            font_names.push(name);
            font_data.push(data);
        }

        let renderer = unsafe { (api.renderer_init)(library) };
        if renderer.is_null() {
            unsafe { (api.library_done)(library) };
            return Err("ass_renderer_init failed".into());
        }

        unsafe {
            (api.set_storage_size)(renderer, width as c_int, height as c_int);
            (api.set_frame_size)(renderer, width as c_int, height as c_int);
            (api.set_fonts)(
                renderer,
                default_font_c.as_ptr(),
                ptr::null(),
                0,
                ptr::null(),
                0,
            );
        }

        let track = unsafe { (api.read_file)(library, ass_path_c.as_ptr(), ptr::null()) };
        if track.is_null() {
            unsafe {
                (api.renderer_done)(renderer);
                (api.library_done)(library);
            }
            return Err(format!("libass could not read {}", ass_path.display()));
        }

        Ok(Self {
            api,
            library,
            renderer,
            track,
            width,
            height,
            _font_names: font_names,
            _font_data: font_data,
            _default_font: default_font_c,
            _ass_path: ass_path_c,
        })
    }
    fn render(&mut self, time_ms: i64) -> Vec<u8> {
        let mut changed = 0;
        let mut image =
            unsafe { (self.api.render_frame)(self.renderer, self.track, time_ms, &mut changed) };
        let mut frame = vec![0u8; self.width * self.height * 4];

        while !image.is_null() {
            let img = unsafe { &*image };
            blend_image(&mut frame, self.width, self.height, img);
            image = img.next;
        }

        unpremultiply(&mut frame);
        frame
    }
}

impl Drop for RenderContext<'_> {
    fn drop(&mut self) {
        unsafe {
            if !self.track.is_null() {
                (self.api.free_track)(self.track);
            }
            if !self.renderer.is_null() {
                (self.api.renderer_done)(self.renderer);
            }
            if !self.library.is_null() {
                (self.api.library_done)(self.library);
            }
        }
    }
}

fn path_cstring(path: &Path) -> Result<CString, String> {
    CString::new(path.to_string_lossy().as_bytes())
        .map_err(|_| format!("path contains a NUL byte: {}", path.display()))
}

fn collect_font_files(dir: &Path) -> Result<Vec<PathBuf>, String> {
    let mut stack = vec![dir.to_path_buf()];
    let mut fonts = Vec::new();

    while let Some(current) = stack.pop() {
        let entries = fs::read_dir(&current)
            .map_err(|e| format!("cannot read font directory {}: {e}", current.display()))?;
        for entry in entries {
            let entry = entry.map_err(|e| format!("cannot read font entry: {e}"))?;
            let path = entry.path();
            if path.is_dir() {
                stack.push(path);
                continue;
            }
            let ext = path
                .extension()
                .and_then(|x| x.to_str())
                .unwrap_or_default()
                .to_ascii_lowercase();
            if matches!(ext.as_str(), "ttf" | "otf" | "pfb") {
                fonts.push(path);
            }
        }
    }

    fonts.sort();
    if fonts.is_empty() {
        return Err(format!(
            "no .ttf, .otf, or .pfb font found in {}",
            dir.display()
        ));
    }
    Ok(fonts)
}

fn blend_image(frame: &mut [u8], width: usize, height: usize, img: &AssImage) {
    if img.bitmap.is_null() || img.w <= 0 || img.h <= 0 || img.stride <= 0 {
        return;
    }

    let r = (img.color >> 24) as u8;
    let g = ((img.color >> 16) & 0xff) as u8;
    let b = ((img.color >> 8) & 0xff) as u8;
    let a = (255 - (img.color & 0xff)) as u8;
    let rounding_offset = 255u32 * 255 / 2;

    for sy in 0..img.h {
        let dy = img.dst_y + sy;
        if dy < 0 || dy >= height as c_int {
            continue;
        }
        let src_row = unsafe { img.bitmap.offset((sy * img.stride) as isize) };

        for sx in 0..img.w {
            let dx = img.dst_x + sx;
            if dx < 0 || dx >= width as c_int {
                continue;
            }

            let coverage = unsafe { *src_row.offset(sx as isize) } as u32;
            let k = coverage * a as u32;
            let base = (dy as usize * width + dx as usize) * 4;

            for (channel, source) in [(0, r), (1, g), (2, b)] {
                let old = frame[base + channel] as u32;
                frame[base + channel] =
                    ((k * source as u32 + (255 * 255 - k) * old + rounding_offset) / (255 * 255))
                        as u8;
            }

            let old_alpha = frame[base + 3] as u32;
            frame[base + 3] =
                ((k * 255 + (255 * 255 - k) * old_alpha + rounding_offset) / (255 * 255)) as u8;
        }
    }
}

fn unpremultiply(frame: &mut [u8]) {
    let (pixels, remainder) = frame.as_chunks_mut::<4>();
    debug_assert!(remainder.is_empty());
    for pixel in pixels {
        let alpha = pixel[3];
        if alpha == 0 {
            pixel[0] = 0;
            pixel[1] = 0;
            pixel[2] = 0;
            continue;
        }

        let offset = 1u32 << 15;
        let inv = ((255u32 << 16) / alpha as u32) + 1;
        for channel in &mut pixel[..3] {
            *channel = ((*channel as u32 * inv + offset) >> 16) as u8;
        }
    }
}

#[derive(Clone, Debug)]
struct Event {
    start_ms: i64,
    end_ms: i64,
    text: String,
}

fn parse_events(source: &str) -> Result<Vec<Event>, String> {
    let mut in_events = false;
    let mut format: Vec<String> = Vec::new();
    let mut events = Vec::new();

    for raw_line in source.lines() {
        let line = raw_line.trim_end_matches('\r');
        let trimmed = line.trim();

        if trimmed.starts_with('[') && trimmed.ends_with(']') {
            in_events = trimmed.eq_ignore_ascii_case("[Events]");
            continue;
        }
        if !in_events {
            continue;
        }

        let Some((key, value)) = line.split_once(':') else {
            continue;
        };
        if key.trim().eq_ignore_ascii_case("Format") {
            format = value
                .split(',')
                .map(|x| x.trim().to_ascii_lowercase())
                .collect();
            continue;
        }
        if !key.trim().eq_ignore_ascii_case("Dialogue") || format.is_empty() {
            continue;
        }

        let fields: Vec<&str> = value.splitn(format.len(), ',').collect();
        if fields.len() != format.len() {
            continue;
        }

        let start_index = format.iter().position(|x| x == "start");
        let end_index = format.iter().position(|x| x == "end");
        let text_index = format.iter().position(|x| x == "text");
        let (Some(start_index), Some(end_index), Some(text_index)) =
            (start_index, end_index, text_index)
        else {
            continue;
        };

        let start_ms = parse_ass_time(fields[start_index])?;
        let end_ms = parse_ass_time(fields[end_index])?;
        if end_ms <= start_ms {
            continue;
        }

        events.push(Event {
            start_ms,
            end_ms,
            text: fields[text_index].to_string(),
        });
    }

    Ok(events)
}

fn parse_ass_time(raw: &str) -> Result<i64, String> {
    let parts: Vec<&str> = raw.trim().split(':').collect();
    if parts.len() != 3 {
        return Err(format!("invalid ASS timestamp: {raw:?}"));
    }

    let hours: i64 = parts[0]
        .parse()
        .map_err(|_| format!("invalid ASS timestamp: {raw:?}"))?;
    let minutes: i64 = parts[1]
        .parse()
        .map_err(|_| format!("invalid ASS timestamp: {raw:?}"))?;

    let (seconds_raw, fraction_raw) = parts[2].split_once('.').unwrap_or((parts[2], ""));
    let seconds: i64 = seconds_raw
        .parse()
        .map_err(|_| format!("invalid ASS timestamp: {raw:?}"))?;

    let mut fraction = fraction_raw.chars().take(3).collect::<String>();
    while fraction.len() < 3 {
        fraction.push('0');
    }
    let millis = if fraction.is_empty() {
        0
    } else {
        fraction
            .parse::<i64>()
            .map_err(|_| format!("invalid ASS timestamp: {raw:?}"))?
    };

    Ok(((hours * 60 + minutes) * 60 + seconds) * 1000 + millis)
}
fn collect_critical_times(events: &[Event]) -> BTreeSet<i64> {
    let transform = Regex::new(r"\\t\(\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*,").unwrap();
    let move_tag = Regex::new(
        r"\\move\(\s*[^,]+,\s*[^,]+,\s*[^,]+,\s*[^,]+,\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*\)",
    )
    .unwrap();
    let fad = Regex::new(r"\\fad\(\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*\)").unwrap();
    let fade = Regex::new(
        r"\\fade\(\s*[^,]+,\s*[^,]+,\s*[^,]+,\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*\)",
    )
    .unwrap();
    let karaoke = Regex::new(r"\\(kt|kf|ko|[kK])(\d+)").unwrap();

    let mut times = BTreeSet::new();

    for event in events {
        add_around(&mut times, event, event.start_ms);
        add_around(&mut times, event, event.start_ms + 1);
        add_around(
            &mut times,
            event,
            event.start_ms + (event.end_ms - event.start_ms) / 2,
        );
        add_around(&mut times, event, event.end_ms - 1);

        for caps in transform.captures_iter(&event.text) {
            let a = parse_relative_ms(&caps[1]);
            let b = parse_relative_ms(&caps[2]);
            add_relative_range(&mut times, event, a, b);
        }

        for caps in move_tag.captures_iter(&event.text) {
            let a = parse_relative_ms(&caps[1]);
            let b = parse_relative_ms(&caps[2]);
            add_relative_range(&mut times, event, a, b);
        }

        for caps in fad.captures_iter(&event.text) {
            let fade_in = parse_relative_ms(&caps[1]);
            let fade_out = parse_relative_ms(&caps[2]);
            add_around(&mut times, event, event.start_ms + fade_in);
            add_around(&mut times, event, event.end_ms - fade_out);
        }

        for caps in fade.captures_iter(&event.text) {
            for i in 1..=4 {
                add_around(
                    &mut times,
                    event,
                    event.start_ms + parse_relative_ms(&caps[i]),
                );
            }
        }

        let mut karaoke_ms = 0i64;
        for caps in karaoke.captures_iter(&event.text) {
            let value = caps[2].parse::<i64>().unwrap_or(0) * 10;
            if &caps[1] == "kt" {
                karaoke_ms = value;
            } else {
                karaoke_ms += value;
            }
            add_around(&mut times, event, event.start_ms + karaoke_ms);
        }
    }

    times
}

fn parse_relative_ms(raw: &str) -> i64 {
    raw.parse::<f64>().map(|v| v.round() as i64).unwrap_or(0)
}

fn add_relative_range(times: &mut BTreeSet<i64>, event: &Event, a: i64, b: i64) {
    add_around(times, event, event.start_ms + a);
    add_around(times, event, event.start_ms + (a + b) / 2);
    add_around(times, event, event.start_ms + b);
}

fn add_around(times: &mut BTreeSet<i64>, event: &Event, time_ms: i64) {
    for t in [time_ms - 1, time_ms, time_ms + 1] {
        if t >= event.start_ms && t < event.end_ms {
            times.insert(t);
        }
    }
}

fn add_frame_times(times: &mut BTreeSet<i64>, events: &[Event], fps: (u64, u64)) {
    let (num, den) = fps;
    let frame_den = 1000u128 * den as u128;

    for event in events {
        let start = event.start_ms.max(0) as u128;
        let end = event.end_ms.max(0) as u128;
        let first = start * num as u128 / frame_den;
        let last = (end * num as u128).div_ceil(frame_den);

        for n in first..=last {
            let center_num = (2 * n + 1) * 1000u128 * den as u128;
            let center_den = 2u128 * num as u128;
            let t = (center_num / center_den) as i64;
            if t >= event.start_ms && t < event.end_ms {
                times.insert(t);
            }
        }
    }
}

#[derive(Debug)]
struct Args {
    before: PathBuf,
    after: PathBuf,
    libass: PathBuf,
    fonts: PathBuf,
    width: usize,
    height: usize,
    explicit_times: Vec<i64>,
    every_frame: Option<(u64, u64)>,
    diff_dir: Option<PathBuf>,
}

fn parse_args() -> Result<Args, String> {
    let raw: Vec<String> = env::args().skip(1).collect();
    if raw.iter().any(|arg| arg == "-h" || arg == "--help") {
        print_usage();
        std::process::exit(0);
    }
    if raw.len() < 2 {
        return Err("before.ass and after.ass are required".into());
    }

    let before = PathBuf::from(&raw[0]);
    let after = PathBuf::from(&raw[1]);
    let mut libass = env::var_os("LIBASS_PATH").map(PathBuf::from);
    let mut fonts = env::var_os("ASSX_RENDERCHECK_FONTS").map(PathBuf::from);
    let mut width = 1920usize;
    let mut height = 1080usize;
    let mut explicit_times = Vec::new();
    let mut every_frame = None;
    let mut diff_dir = None;

    let mut i = 2;
    while i < raw.len() {
        let flag = &raw[i];
        let value = |i: &mut usize| -> Result<&str, String> {
            *i += 1;
            raw.get(*i)
                .map(String::as_str)
                .ok_or_else(|| format!("{flag} requires a value"))
        };

        match flag.as_str() {
            "--libass" => libass = Some(PathBuf::from(value(&mut i)?)),
            "--fonts" => fonts = Some(PathBuf::from(value(&mut i)?)),
            "--width" => {
                width = value(&mut i)?
                    .parse()
                    .map_err(|_| "--width must be a positive integer".to_string())?
            }
            "--height" => {
                height = value(&mut i)?
                    .parse()
                    .map_err(|_| "--height must be a positive integer".to_string())?
            }
            "--time-ms" => {
                explicit_times.push(
                    value(&mut i)?
                        .parse()
                        .map_err(|_| "--time-ms must be an integer".to_string())?,
                );
            }
            "--every-frame" => every_frame = Some(parse_fps(value(&mut i)?)?),
            "--diff-dir" => diff_dir = Some(PathBuf::from(value(&mut i)?)),
            other => return Err(format!("unknown argument: {other}")),
        }
        i += 1;
    }

    if width == 0 || height == 0 {
        return Err("width and height must be positive".into());
    }

    let libass = libass.ok_or_else(|| {
        "set --libass PATH or LIBASS_PATH to the pinned libass shared library".to_string()
    })?;
    let fonts = fonts.ok_or_else(|| {
        "set --fonts DIR or ASSX_RENDERCHECK_FONTS to a deterministic font directory".to_string()
    })?;

    Ok(Args {
        before,
        after,
        libass,
        fonts,
        width,
        height,
        explicit_times,
        every_frame,
        diff_dir,
    })
}

fn parse_fps(raw: &str) -> Result<(u64, u64), String> {
    let (num, den) = match raw.split_once('/') {
        Some((n, d)) => (
            n.parse::<u64>()
                .map_err(|_| format!("invalid fps: {raw}"))?,
            d.parse::<u64>()
                .map_err(|_| format!("invalid fps: {raw}"))?,
        ),
        None => (
            raw.parse::<u64>()
                .map_err(|_| format!("invalid fps: {raw}"))?,
            1,
        ),
    };
    if num == 0 || den == 0 {
        return Err("fps numerator and denominator must be positive".into());
    }
    Ok((num, den))
}

fn print_usage() {
    eprintln!(
        "usage: assx-rendercheck BEFORE.ass AFTER.ass \
  --libass PATH --fonts DIR [--width 1920 --height 1080] \
  [--time-ms N]... [--every-frame 24000/1001] [--diff-dir DIR]"
    );
}
#[derive(Debug)]
struct DiffStats {
    changed_pixels: usize,
    max_channel_delta: u8,
    bbox: Option<(usize, usize, usize, usize)>,
}

fn diff_stats(a: &[u8], b: &[u8], width: usize) -> DiffStats {
    let mut changed_pixels = 0usize;
    let mut max_channel_delta = 0u8;
    let mut bbox: Option<(usize, usize, usize, usize)> = None;

    let (a_pixels, a_remainder) = a.as_chunks::<4>();
    let (b_pixels, b_remainder) = b.as_chunks::<4>();
    debug_assert!(a_remainder.is_empty() && b_remainder.is_empty());

    for (index, (pa, pb)) in a_pixels.iter().zip(b_pixels.iter()).enumerate() {
        let mut changed = false;
        for channel in 0..4 {
            let delta = pa[channel].abs_diff(pb[channel]);
            max_channel_delta = max_channel_delta.max(delta);
            changed |= delta != 0;
        }
        if !changed {
            continue;
        }

        changed_pixels += 1;
        let x = index % width;
        let y = index / width;
        bbox = Some(match bbox {
            None => (x, y, x, y),
            Some((x1, y1, x2, y2)) => (x1.min(x), y1.min(y), x2.max(x), y2.max(y)),
        });
    }

    DiffStats {
        changed_pixels,
        max_channel_delta,
        bbox,
    }
}

fn write_png(path: &Path, rgba: &[u8], width: usize, height: usize) -> Result<(), String> {
    let file =
        fs::File::create(path).map_err(|e| format!("cannot create {}: {e}", path.display()))?;
    let mut encoder = png::Encoder::new(file, width as u32, height as u32);
    encoder.set_color(png::ColorType::Rgba);
    encoder.set_depth(png::BitDepth::Eight);
    let mut writer = encoder
        .write_header()
        .map_err(|e| format!("cannot write {}: {e}", path.display()))?;
    writer
        .write_image_data(rgba)
        .map_err(|e| format!("cannot write {}: {e}", path.display()))
}

fn make_diff_image(a: &[u8], b: &[u8]) -> Vec<u8> {
    let mut out = vec![0u8; a.len()];
    let (a_pixels, a_remainder) = a.as_chunks::<4>();
    let (b_pixels, b_remainder) = b.as_chunks::<4>();
    let (out_pixels, out_remainder) = out.as_chunks_mut::<4>();
    debug_assert!(a_remainder.is_empty() && b_remainder.is_empty() && out_remainder.is_empty());

    for ((pa, pb), dst) in a_pixels
        .iter()
        .zip(b_pixels.iter())
        .zip(out_pixels.iter_mut())
    {
        let changed = pa != pb;
        for channel in 0..3 {
            dst[channel] = pa[channel].abs_diff(pb[channel]).saturating_mul(8);
        }
        dst[3] = if changed { 255 } else { 0 };
    }
    out
}

fn run() -> Result<i32, String> {
    let args = parse_args()?;
    let before_source = fs::read_to_string(&args.before)
        .map_err(|e| format!("cannot read {}: {e}", args.before.display()))?;
    let after_source = fs::read_to_string(&args.after)
        .map_err(|e| format!("cannot read {}: {e}", args.after.display()))?;

    let before_events = parse_events(&before_source)?;
    let after_events = parse_events(&after_source)?;

    let mut times = collect_critical_times(&before_events);
    times.extend(collect_critical_times(&after_events));
    times.extend(args.explicit_times.iter().copied());

    if let Some(fps) = args.every_frame {
        add_frame_times(&mut times, &before_events, fps);
        add_frame_times(&mut times, &after_events, fps);
    }

    if times.is_empty() {
        return Err("no render times were found; add a Dialogue event or use --time-ms".into());
    }

    if let Some(dir) = &args.diff_dir {
        fs::create_dir_all(dir)
            .map_err(|e| format!("cannot create diff directory {}: {e}", dir.display()))?;
    }

    let api = unsafe { Api::load(&args.libass)? };
    let mut before =
        unsafe { RenderContext::new(&api, &args.before, &args.fonts, args.width, args.height)? };
    let mut after =
        unsafe { RenderContext::new(&api, &args.after, &args.fonts, args.width, args.height)? };

    let mut mismatches = 0usize;

    for time_ms in &times {
        let before_frame = before.render(*time_ms);
        let after_frame = after.render(*time_ms);

        if before_frame == after_frame {
            continue;
        }

        mismatches += 1;
        let stats = diff_stats(&before_frame, &after_frame, args.width);
        let bbox = stats
            .bbox
            .map(|(x1, y1, x2, y2)| format!("{x1},{y1}..{x2},{y2}"))
            .unwrap_or_else(|| "none".to_string());

        eprintln!(
            "mismatch at {time_ms} ms: {} changed pixels, max channel delta {}, bbox {}",
            stats.changed_pixels, stats.max_channel_delta, bbox
        );

        if let Some(dir) = &args.diff_dir {
            write_png(
                &dir.join(format!("{time_ms}ms-before.png")),
                &before_frame,
                args.width,
                args.height,
            )?;
            write_png(
                &dir.join(format!("{time_ms}ms-after.png")),
                &after_frame,
                args.width,
                args.height,
            )?;
            write_png(
                &dir.join(format!("{time_ms}ms-diff.png")),
                &make_diff_image(&before_frame, &after_frame),
                args.width,
                args.height,
            )?;
        }
    }

    if mismatches == 0 {
        println!(
            "render-equivalent: {} exact RGBA frames matched ({}x{})",
            times.len(),
            args.width,
            args.height
        );
        Ok(0)
    } else {
        eprintln!(
            "render mismatch: {mismatches} of {} sampled frames differ",
            times.len()
        );
        Ok(1)
    }
}

fn main() {
    match run() {
        Ok(code) => std::process::exit(code),
        Err(error) => {
            eprintln!("rendercheck error: {error}");
            std::process::exit(2);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ass_time_without_float_rounding() {
        assert_eq!(parse_ass_time("0:00:01.23").unwrap(), 1230);
        assert_eq!(parse_ass_time("1:02:03.004").unwrap(), 3_723_004);
        assert_eq!(parse_ass_time("0:00:00.5").unwrap(), 500);
    }

    #[test]
    fn parses_dialogue_using_event_format() {
        let source = "[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,{\\fs20}a,b\n";
        let events = parse_events(source).unwrap();
        assert_eq!(events.len(), 1);
        assert_eq!(events[0].start_ms, 1000);
        assert_eq!(events[0].end_ms, 2000);
        assert_eq!(events[0].text, "{\\fs20}a,b");
    }

    #[test]
    fn critical_times_include_animation_boundaries() {
        let events = vec![Event {
            start_ms: 1000,
            end_ms: 5000,
            text: r"{\t(500,1500,\fs40)\fad(200,300)\k20\k30}x".into(),
        }];
        let times = collect_critical_times(&events);

        for expected in [
            1199, 1200, 1201, 1499, 1500, 1501, 1999, 2000, 2001, 2499, 2500, 2501, 4699, 4700,
            4701,
        ] {
            assert!(times.contains(&expected), "missing {expected} in {times:?}");
        }
    }

    #[test]
    fn every_frame_adds_frame_centers_inside_event() {
        let events = vec![Event {
            start_ms: 0,
            end_ms: 100,
            text: "x".into(),
        }];
        let mut times = BTreeSet::new();
        add_frame_times(&mut times, &events, (25, 1));
        assert!(times.contains(&20));
        assert!(times.contains(&60));
    }
}
