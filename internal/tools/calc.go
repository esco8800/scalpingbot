package tools

import (
	"scalpingbot/internal/exchange"
	"strconv"
)

func CalculateSellVolumeInUSDT(orders []exchange.OrderInfo) float64 {
	var totalSellVolumeUSDT float64

	for _, order := range orders {
		if order.Side == exchange.Sell && order.Status == exchange.Filled {
			executedQty, err1 := strconv.ParseFloat(order.ExecutedQty, 64)
			price, err2 := strconv.ParseFloat(order.Price, 64)

			if err1 == nil && err2 == nil {
				totalSellVolumeUSDT += executedQty * price
			}
		}
	}

	return totalSellVolumeUSDT
}
