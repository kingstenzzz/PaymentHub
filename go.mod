module github.com/kingstenzzz/PaymentHub

go 1.16

require (
	github.com/ethereum/go-ethereum v1.10.16
	github.com/kingstenzzz/PaymentHub/Nocust v0.0.0-20220223093632-e8b07147d1e4
	github.com/kingstenzzz/PaymentHub/PayHub v1.0.0
	github.com/kingstenzzz/PaymentHub/TURBO v0.0.0-20220223093632-e8b07147d1e4
	github.com/kingstenzzz/PaymentHub/Utils v1.0.0
	github.com/wealdtech/go-merkletree v1.0.0
)

replace github.com/kingstenzzz/PaymentHub/PayHub v1.0.0 => ./PayHub

replace github.com/kingstenzzz/PaymentHub/TURBO v0.0.0-20220223093632-e8b07147d1e4 => ./TURBO

replace github.com/kingstenzzz/PaymentHub/Utils v1.0.0 => ./Utils
