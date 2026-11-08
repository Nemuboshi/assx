export interface ServerSettings {
  renderers: string[];
}

export function serverSettings(read: (key: string) => unknown): ServerSettings {
  return { renderers: read('renderers') as string[] };
}
