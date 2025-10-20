package config

// TODO: Add fields and tags for environment loading and validation.
type HTTPConfig struct {
    // TODO: address/port, timeouts, CORS
}

type DBConfig struct {
    // TODO: connection URL, pool sizes, timeouts
}

type RedisConfig struct {
    // TODO: address, password, db index
}

type AuthConfig struct {
    // TODO: JWT issuer, public/private keys, API Key settings
}

type ObservabilityConfig struct {
    // TODO: log level, metrics toggle/port, tracing exporter
}

type Config struct {
    HTTP           HTTPConfig
    DB             DBConfig
    Redis          RedisConfig
    Auth           AuthConfig
    Observability  ObservabilityConfig
}

// TODO: Load() to read from environment and validate configuration.
