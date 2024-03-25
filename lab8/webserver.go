// Modified for lab 8 - docker hosting

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongodbEndpoint = "mongodb://localhost:27017" // Find this from the Mongo container
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
	mu      sync.Mutex
}

func newDatabase() *database {

	client, err := mongo.NewClient(
		options.Client().ApplyURI(mongodbEndpoint),
	)
	checkError(err)

	// Connect to mongo
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)

	// Disconnect
	defer client.Disconnect(ctx)

	// select collection from database
	col := client.Database("blog").Collection("posts")

	return &database{
		data:    col,
		connect: ctx,
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
		fmt.Printf("%s: %d\n", p.Item, p.Price)
	}
}

func (db database) price(w http.ResponseWriter, r *http.Request) {
	item := r.URL.Query().Get("item")

	// find one document
	var p Post
	err := db.data.FindOne(context.Background(), bson.M{"item": item}).Decode(&p)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "item not found\n")
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error finding item: %v\n", err)
		return
	}
	fmt.Printf("price of %s: %d\n", item, p.Price)
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

	_, err := db.data.InsertOne(db.connect, &Post{
		ID:        primitive.NewObjectID(),
		Item:      "shoes",
		Price:     50,
		CreatedAt: time.Now(),
	})

	_, err = db.data.InsertOne(db.connect, &Post{
		ID:        primitive.NewObjectID(),
		Item:      "socks",
		Price:     5,
		CreatedAt: time.Now(),
	})

	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/list", http.HandlerFunc(db.list))
	mux.Handle("/price", http.HandlerFunc(db.price))
	mux.Handle("/create", http.HandlerFunc(db.create))
	mux.Handle("/update", http.HandlerFunc(db.update))
	mux.Handle("/remove", http.HandlerFunc(db.remove))
	log.Fatal(http.ListenAndServe("localhost:8000", mux))
}
