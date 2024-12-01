package entity

type Config struct {
	PostgresConfig PostgresConfig `yaml:"database"`
	JWTSecretKey   []byte         `yaml:"jwt_secret"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	Port     string `yaml:"port"`
	SSLMode  string `yaml:"sslmode"`
}
