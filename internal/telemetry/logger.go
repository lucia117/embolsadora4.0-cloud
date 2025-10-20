package telemetry

import (
    "os"

    "go.uber.org/zap"
)

// TODO: accept a real config.Config and use fields instead of env lookup.
func NewLogger(cfg interface{}) (*zap.Logger, error) {
    if os.Getenv("APP_ENV") == "dev" {
        return zap.NewDevelopment()
    }
    // TODO: production logger configuration (sampling, encoding, fields).
    return zap.NewNop(), nil
}
