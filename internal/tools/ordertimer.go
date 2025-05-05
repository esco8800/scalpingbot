package tools

import (
	"log"
	"math"

	"scalpingbot/internal/exchange"
)

func AdjustTimeout(baseTimeout int, klines []exchange.Kline) int {
	if len(klines) == 0 || baseTimeout <= 0 {
		return baseTimeout
	}

	redCount := 0
	greenCount := 0

loop:
	for i := len(klines) - 1; i >= 0; i-- {
		k := klines[i]
		switch {
		case k.Close < k.Open:
			log.Printf("kline #%d: close < open, redCount: %d", i, redCount)
			if greenCount > 0 {
				break loop
			}
			redCount++
		case k.Close > k.Open:
			log.Printf("kline #%d: close > open, greenCount: %d", i, greenCount)
			if redCount > 0 {
				break loop
			}
			greenCount++
		default:
			log.Printf("kline #%d: close == open, break loop", i)
			break loop
		}
	}

	if redCount > 0 {
		return int(math.Round(float64(baseTimeout) + float64(baseTimeout*redCount)))
	}
	if greenCount > 0 {
		return int(math.Round(float64(baseTimeout) / float64(greenCount)))
	}

	return baseTimeout
}
