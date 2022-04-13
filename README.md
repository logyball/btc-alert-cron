# BTC Alerts Cronjob

Running: `make start`

Mongo Document structure:

```go
{
	ID     primitive.ObjectID `bson:"_id"`
	Name   string             `bson:"name"`
	Type   string             `bson:"type"`
	Price  float32            `bson:"price"`
	Status string             `bson:"status"`
}
```