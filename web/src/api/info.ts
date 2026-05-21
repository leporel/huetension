import { api, type RequestOptions } from './client';

export interface ServerInfo {
  name: string;
  version: string;
  schema: string;
}

export function getInfo(opts?: RequestOptions): Promise<ServerInfo> {
  return api.get<ServerInfo>('/info', undefined, opts);
}
