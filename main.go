package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/robfig/cron"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"

	log "github.com/sirupsen/logrus"
)

const (
	coinbaseUrl      string = "https://api.coinbase.com/v2/prices/%s/%s" // currency pair, verb (buy|sell)
	btcToUSD         string = "BTC-USD"
	dbName           string = "btc"
	collectionName   string = "alerts"
	runEveryXSeconds int    = 20
)

var buyUrl string = fmt.Sprintf(coinbaseUrl, btcToUSD, "buy")
var sellUrl string = fmt.Sprintf(coinbaseUrl, btcToUSD, "sell")

var mongodbUri string

type CoinbaseResponse struct {
	Data map[string]string `json:"data"`
}

type Alert struct {
	ID     primitive.ObjectID `bson:"_id"`
	Name   string             `bson:"name"`
	Type   string             `bson:"type"`
	Price  float32            `bson:"price"`
	Status string             `bson:"status"`
}

func getPrices() (float32, float32) {
	log.Info("getting BTC prices from coinbase API")
	var cBuyResp, cSellResp CoinbaseResponse

	buyResp, err := http.Get(buyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	defer buyResp.Body.Close()

	sellResp, err := http.Get(sellUrl)
	if err != nil {
		log.Fatalln(err)
	}
	defer sellResp.Body.Close()

	err = json.NewDecoder(buyResp.Body).Decode(&cBuyResp)
	if err != nil {
		log.Fatalln(err)
	}

	err = json.NewDecoder(sellResp.Body).Decode(&cSellResp)
	if err != nil {
		log.Fatalln(err)
	}

	buyPrice, err := strconv.ParseFloat(cBuyResp.Data["amount"], 32)
	if err != nil {
		log.Fatalln(err)
	}

	sellPrice, err := strconv.ParseFloat(cSellResp.Data["amount"], 32)
	if err != nil {
		log.Fatalln(err)
	}

	log.Infof("Current BTC buy price: %f, current BTC sell Price: %f", buyPrice, sellPrice)

	return float32(buyPrice), float32(sellPrice)
}

func checkAlerts(ctx context.Context, alerts *mongo.Collection) []*Alert {
	log.Info("checking alerts in collection")

	cursor, err := alerts.Find(ctx, bson.M{"status": bson.M{"$eq": "ACTIVE"}})
	if err != nil {
		log.Fatal(err)
	}

	var alertList []*Alert
	err = cursor.All(ctx, &alertList)
	if err != nil {
		log.Fatal(err)
	}

	if len(alertList) == 0 {
		log.Info("No alerts to fire")
	}

	return alertList
}

func fireAlert(ctx context.Context, alertsCollection *mongo.Collection, alert *Alert) {
	log.Infof("Firing alert for ID %s - Name %s", alert.ID, alert.Name)

	/*
		NOTIFICATION PLACEHOLDER
	*/
	result, err := alertsCollection.UpdateByID(ctx, alert.ID, bson.M{"$set": bson.M{"status": "FIRED"}})
	if err != nil {
		log.Fatal(err)
	}
	if result.MatchedCount < 1 {
		log.Warnf("Failed to update document %s\n", alert.ID)
	} else {
		log.Infof("Updated document %s to FIRED", alert.ID)
	}
}

func scanAlerts(ctx context.Context, alertsCollection *mongo.Collection, alertList []*Alert, buyPrice float32, sellPrice float32) {
	for _, alert := range alertList {
		if alert.Type == "MAX" && (sellPrice > alert.Price) {
			log.Infof("Triggered max alert %s at threshold %v for price %v\n", alert.Name, alert.Price, sellPrice)
			fireAlert(ctx, alertsCollection, alert)
		}
		if alert.Type == "MIN" && (buyPrice < alert.Price) {
			log.Infof("Triggered min alert %s at threshold %v for price %v\n", alert.Name, alert.Price, buyPrice)
			fireAlert(ctx, alertsCollection, alert)
		}
	}
}

func Wrapper() {
	truePtr := true
	opts := &options.ClientOptions{
		RetryWrites: &truePtr,
	}
	client, err := mongo.NewClient(opts.ApplyURI(mongodbUri))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	alertsCollection := db.Collection(collectionName)

	buyPrice, sellPrice := getPrices()
	alertList := checkAlerts(ctx, alertsCollection)
	scanAlerts(ctx, alertsCollection, alertList, buyPrice, sellPrice)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("could not load dotenv")
	}

	mongodbUri = os.Getenv("MONGO_CONNECTION")
	if mongodbUri == "" {
		log.Fatal("unable to read mongo uri from environment")
	}

	fmt.Println(mongodbUri)

}

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	c := cron.New()
	c.AddFunc(fmt.Sprintf("*/%d * * * *", runEveryXSeconds), Wrapper)

	log.Infof("Starting cron job, running every %d seconds", runEveryXSeconds)
	c.Start()

	select {} // run until terminated
}
