package bot

// Order - структура ордера
type Order struct {
	BuyPrice  float64
	SellPrice float64
	Amount    float64
	Active    bool
}

// NewOrder - создание нового ордера
func NewOrder(buyPrice, profitPercent, orderSize float64) Order {
	amount := orderSize / buyPrice
	sellPrice := buyPrice * (1 + profitPercent/100)
	return Order{
		BuyPrice:  buyPrice,
		SellPrice: sellPrice,
		Amount:    amount,
		Active:    true,
	}
}
