export interface ServerSettings {
  primaryRenderer: string;
  compatibilityRenderers: string[];
  vsfilterMod: { mod: string; lua: string };
}

export function serverSettings(read: (key: string) => unknown): ServerSettings {
  return {
    primaryRenderer: read('primaryRenderer') as string,
    compatibilityRenderers: read('compatibilityRenderers') as string[],
    vsfilterMod: {
      mod: read('vsfilterMod.mod') as string,
      lua: read('vsfilterMod.lua') as string
    }
  };
}
