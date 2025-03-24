package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config - структура конфигурации бота
type Config struct {
	ProfitPercent    float64 `mapstructure:"profit_percent"`
	DropPercent      float64 `mapstructure:"drop_percent"`
	DelaySeconds     int     `mapstructure:"delay_seconds"`
	OrderSize        float64 `mapstructure:"order_size"`
	Deposit          float64 `mapstructure:"deposit"`
	AvailableDeposit float64 `mapstructure:"-"` // Не читаем из конфига, вычисляем
	APIKey           string  `mapstructure:"api_key"`
	SecretKey        string  `mapstructure:"secret_key"`
	Symbol           string  `mapstructure:"symbol"` // Например, "KASUSDT"
}

// LoadConfig - загрузка конфигурации через Viper
func LoadConfig() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Значения по умолчанию
	viper.SetDefault("profit_percent", 0.3)
	viper.SetDefault("drop_percent", 1.0)
	viper.SetDefault("delay_seconds", 30)
	viper.SetDefault("order_size", 40.0)
	viper.SetDefault("deposit", 400.0)
	viper.SetDefault("api_key", "")
	viper.SetDefault("secret_key", "")
	viper.SetDefault("symbol", "KASUSDT") // Kaspa как пример

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Конфигурационный файл не найден, используются значения по умолчанию")
		} else {
			return Config{}, fmt.Errorf("ошибка чтения конфигурации: %v", err)
		}
	}

	viper.AutomaticEnv()

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("ошибка разбора конфигурации: %v", err)
	}

	// Проверяем обязательные поля
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("API Key и Secret Key обязательны для MEXC")
	}

	cfg.AvailableDeposit = cfg.Deposit
	return cfg, nil
}
