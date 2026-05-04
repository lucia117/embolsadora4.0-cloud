package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvBeta       Environment = "beta"
	EnvProduction Environment = "production"
)

func (e Environment) IsProduction() bool { return e == EnvProduction }
func (e Environment) IsLocal() bool      { return e == EnvLocal }
func (e Environment) IsBeta() bool       { return e == EnvBeta }

// LoadEnvFile detecta APP_ENV y carga el archivo .env.<ambiente> correspondiente.
// Las variables de sistema tienen prioridad sobre el archivo (godotenv no sobreescribe).
func LoadEnvFile() Environment {
	env := Environment(getEnv("APP_ENV", string(EnvLocal)))
	switch env {
	case EnvLocal, EnvBeta, EnvProduction:
	default:
		log.Printf("APP_ENV=%q desconocido, usando %q", env, EnvLocal)
		env = EnvLocal
	}

	file := fmt.Sprintf(".env.%s", env)
	if err := godotenv.Load(file); err != nil {
		log.Printf("archivo %s no encontrado, usando variables de sistema", file)
	} else {
		log.Printf("configuración cargada desde %s (ambiente: %s)", file, env)
	}
	return env
}

type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DBConfig struct {
	URL             string
	MaxConns        int
	MinConns        int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL string
}

type SupabaseConfig struct {
	JWKSUrl             string
	JWTIssuer           string
	JWTAudience         string
	URL                 string
	ServiceRoleKey      string
	AnonKey             string
	AppBaseURL          string
	InviteRateLimitHour int
}

type ObservabilityConfig struct {
	LogLevel string
}

type Config struct {
	Env           Environment
	HTTP          HTTPConfig
	DB            DBConfig
	Redis         RedisConfig
	Supabase      SupabaseConfig
	Observability ObservabilityConfig
}

func Load() (*Config, error) {
	var missing []string
	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		Env: Environment(getEnv("APP_ENV", string(EnvLocal))),
		HTTP: HTTPConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second),
		},
		DB: DBConfig{
			URL:             require("DATABASE_URL"),
			MaxConns:        getIntEnv("DB_MAX_CONNS", 10),
			MinConns:        getIntEnv("DB_MIN_CONNS", 2),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", ""),
		},
		Supabase: SupabaseConfig{
			JWKSUrl:             require("SUPABASE_JWKS_URL"),
			JWTIssuer:           require("SUPABASE_JWT_ISSUER"),
			JWTAudience:         getEnv("SUPABASE_JWT_AUDIENCE", "authenticated"),
			URL:                 require("SUPABASE_URL"),
			ServiceRoleKey:      require("SUPABASE_SERVICE_ROLE_KEY"),
			AnonKey:             getEnv("SUPABASE_ANON_KEY", ""),
			AppBaseURL:          require("APP_BASE_URL"),
			InviteRateLimitHour: getIntEnv("INVITATION_RATE_LIMIT_PER_HOUR", 20),
		},
		Observability: ObservabilityConfig{
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}


func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
