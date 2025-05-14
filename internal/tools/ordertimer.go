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
			if greenCount > 0 {
				break loop
			}
			redCount++
			log.Printf("kline #%d: close < open, redCount: %d", i, redCount)
		case k.Close > k.Open:
			if redCount > 0 {
				break loop
			}
			greenCount++
			log.Printf("kline #%d: close > open, greenCount: %d", i, greenCount)
		default:
			// если красных больше, то увеличиваем красный счетчик
			if redCount > greenCount {
				redCount++
				continue loop
			}
			// если зеленых больше, то увеличиваем зеленый счетчик
			if greenCount > redCount {
				greenCount++
				continue loop
			}
			log.Printf("kline #%d: close == open, break loop", i)
			break loop
		}
	}

	// начиная с 2 свеч редактируем таймаут
	// если красных больше 1, то уменьшаем таймаут, если зеленые, то возвращаем базовый таймаут
	if redCount > 1 {
		return int(math.Round(float64(baseTimeout) / float64(greenCount)))
	}
	if greenCount > 1 {
		//return int(math.Round(float64(baseTimeout) + float64(baseTimeout*redCount)))
		return baseTimeout
	}

	return baseTimeout
}
