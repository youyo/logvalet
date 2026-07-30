package space

type StoreType string

const (
	StoreTypeMemory   StoreType = "memory"
	StoreTypeSQLite   StoreType = "sqlite"
	StoreTypeDynamoDB StoreType = "dynamodb"
)
