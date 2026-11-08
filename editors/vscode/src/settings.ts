export interface ServerSettings {
  primaryRenderer: string;
  compatibilityRenderers: string[];
}

export function serverSettings(read: (key: string) => unknown): ServerSettings {
  return {
    primaryRenderer: read('primaryRenderer') as string,
    compatibilityRenderers: read('compatibilityRenderers') as string[]
  };
}
