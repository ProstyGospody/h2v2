export type Hy2ConfigValidation = {
  valid: boolean;
  errors: string[];
  warnings: string[];
  raw_only_paths?: string[];
};

export type Hy2ServerBandwidth = {
  up?: string;
  down?: string;
};

export type Hy2Settings = {
  listen: string;
  tlsEnabled: boolean;
  tlsMode: "acme" | "tls" | "conflict" | string;
  clientTLSInsecure?: boolean;
  tls?: { cert?: string; key?: string };
  acme?: { domains?: string[]; email?: string };
  obfs?: { type?: string; salamander?: { password?: string } };
  masquerade?: {
    type?: string;
    listenHTTP?: string;
    listenHTTPS?: string;
    forceHTTPS?: boolean;
    proxy?: { url?: string; rewriteHost?: boolean; insecure?: boolean };
    file?: { dir?: string };
    string?: { content?: string; statusCode?: number; headers?: Record<string, string> };
  };
  bandwidth?: Hy2ServerBandwidth;
  ignoreClientBandwidth?: boolean;
  speedTest?: boolean;
  disableUDP?: boolean;
  udpIdleTimeout?: string;
  quicEnabled: boolean;
  quic?: {
    initStreamReceiveWindow?: number;
    maxStreamReceiveWindow?: number;
    initConnReceiveWindow?: number;
    maxConnReceiveWindow?: number;
    maxIncomingStreams?: number;
    maxIdleTimeout?: string;
    disablePathMTUDiscovery?: boolean;
  };
};

export type Hy2SettingsValidation = {
  valid: boolean;
  errors: string[];
  warnings: string[];
};

export type HysteriaSettingsResponse = {
  path: string;
  raw_yaml: string;
  settings: Hy2Settings;
  settings_validation: Hy2SettingsValidation;
  config_validation: Hy2ConfigValidation;
  raw_only_paths?: string[];
  access_mode: string;
  access_warning: string;
};

export type HysteriaSettingsSaveResponse = {
  ok: boolean;
  path: string;
  backup_path?: string;
  raw_yaml: string;
  settings: Hy2Settings;
  settings_validation: Hy2SettingsValidation;
  config_validation: Hy2ConfigValidation;
  raw_only_paths?: string[];
  access_mode: string;
  access_warning: string;
};
