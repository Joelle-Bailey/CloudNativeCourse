// Modified for lab 8 - docker hosting

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongodbEndpoint = "mongodb://172.18.0.2:27017" // Find this from the Mongo container
)

type dollars float32

func (d dollars) String() string { return fmt.Sprintf("$%.2f", d) }

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Post struct {
	ID        primitive.ObjectID `bson:"_id"`
	Item      string             `bson:"item"`
	Price     dollars            `bson:"price"`
	Tags      []string           `bson:"tags"`
	Comments  uint64             `bson:"comments"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type database struct {
	data    *mongo.Collection
	connect context.Context
	client  *mongo.Client
}

func retry(ctx context.Context, maxAttempts int, interval time.Duration, operation func() error) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := operation()
		if err == nil {
			// Operation succeeded, no need to retry
			return nil
		}

		// Check if the context is done (cancelled or timed out)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// If this is the last attempt, return the error without retrying
		if attempt == maxAttempts {
			return err
		}

		// Sleep for the specified interval before retrying
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func newDatabase() *database {

	operation := func() error {
		// Simulate an operation that may fail
		if time.Now().Second()%2 == 0 {
			return nil // Success
		} else {
			return errors.New("operation failed")
		}
	}

	client, err := mongo.NewClient(
		options.Client().ApplyURI(mongodbEndpoint),
	)
	checkError(err)

	// Connect to mongo
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	err = client.Connect(ctx)

	err = retry(ctx, 4, time.Second, operation)

	if err != nil {
		fmt.Printf("Operation failed after retries: %v\n", err)
	} else {
		fmt.Println("Operation succeeded")
	}

	// select collection from database
	col := client.Database("inventory").Collection("items")

	return &database{
		data:    col,
		connect: ctx,
		client:  client,
	}
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) { // adapter function
	f(w, r)
}

func (db database) list(w http.ResponseWriter, r *http.Request) {

	filter := bson.M{"item": bson.M{"$exists": true}}

	// find all documents
	cursor, err := db.data.Find(db.connect, filter)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(db.connect)

	var posts []Post
	if err := cursor.All(db.connect, &posts); err != nil {
		log.Fatal(err)
	}

	for _, p := range posts {
		fmt.Fprintf(w, "%s: %f\n", p.Item, p.Price)
	}
}

func (db database) price(w http.ResponseWriter, r *http.Request) {
	item := r.URL.Query().Get("item")

	// find one document
	var p Post
	err := db.data.FindOne(context.Background(), bson.M{"item": item}).Decode(&p)
	if err == nil {
		if err == mongo.ErrNoDocuments {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "item not found\n")
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error finding item: %v\n", err)
		return
	}
	fmt.Fprintf(w, "price of %s: %d\n", item, p.Price)
}

func (db database) create(w http.ResponseWriter, r *http.Request) {

	item := r.URL.Query().Get("item")
	price := r.URL.Query().Get("price")

	price_update, ok := strconv.ParseFloat(price, 64)

	if ok != nil {
		w.WriteHeader(http.StatusNotFound) //404 page
		fmt.Fprintf(w, "could not convert price: %q\n", item)
		return
	}

	filter := bson.M{"item": item}
	found := db.data.FindOne(context.Background(), filter)

	if found.Err() == nil {
		if found.Err() != mongo.ErrNoDocuments {
			// Item not found
			// Respond with 404 Not Found
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "item alreay in inventory: %q\n", item)
			return
		}
		// Other error occurred
		// Respond with 500 Internal Server Error
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error checking item: %v\n", found.Err())
		return
	}

	res, err := db.data.InsertOne(db.connect, &Post{
		ID:        primitive.NewObjectID(),
		Item:      item,
		Price:     dollars(price_update),
		CreatedAt: time.Now(),
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error creating item: %s\n", err.Error())
		return
	}

	fmt.Printf("inserted id: %s\n", res.InsertedID.(primitive.ObjectID).Hex())

	fmt.Fprintf(w, "Created item: %s at $%.2f price\n", item, price_update)

}

func (db database) update(w http.ResponseWriter, r *http.Request) {
	item := r.URL.Query().Get("item")
	price := r.URL.Query().Get("price")

	// Create a filter to find the item document
	filter := bson.M{"item": item}

	// Create an update to set the price field
	update := bson.M{"$set": bson.M{"price": price}}

	// Perform the update operation
	_, err := db.data.UpdateOne(context.Background(), filter, update)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error updating item: %v\n", err)
		return
	}

	// Send a success response
	fmt.Fprintf(w, "price of item %q updated to %q\n", item, price)
}

func (db database) remove(w http.ResponseWriter, r *http.Request) {
	item := r.URL.Query().Get("item")

	// Create a filter to find the item document
	filter := bson.M{"item": item}

	// Perform the delete operation
	result, err := db.data.DeleteOne(context.Background(), filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting item: %v\n", err)
		return
	}

	// Check if the item was found and deleted
	if result.DeletedCount == 0 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "no such item: %q\n", item)
		return
	}

	// Send a success response
	fmt.Fprintf(w, "Deleted item: %s ", item)

}

func main() {

	db := newDatabase()

	mux := http.NewServeMux()
	mux.Handle("/list", http.HandlerFunc(db.list))
	mux.Handle("/price", http.HandlerFunc(db.price))
	mux.Handle("/create", http.HandlerFunc(db.create))
	mux.Handle("/update", http.HandlerFunc(db.update))
	mux.Handle("/remove", http.HandlerFunc(db.remove))
	log.Fatal(http.ListenAndServe("localhost:8000", mux))

	//defer db.client.Disconnect(db.connect)
}
