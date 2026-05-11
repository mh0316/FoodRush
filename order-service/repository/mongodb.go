package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	pb "foodrush/orders/proto"
)

var ErrNotFound = errors.New("not found")

type MongoDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

func NewMongoDB(uri, dbName, collName string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	collection := client.Database(dbName).Collection(collName)
	return &MongoDB{client: client, collection: collection}, nil
}

func (db *MongoDB) CreateOrder(ctx context.Context, order *pb.Order) error {
	_, err := db.collection.InsertOne(ctx, order)
	return err
}

func (db *MongoDB) GetOrder(ctx context.Context, id string) (*pb.Order, error) {
	var order pb.Order
	err := db.collection.FindOne(ctx, bson.M{"id": id}).Decode(&order)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (db *MongoDB) UpdateOrderStatus(ctx context.Context, qrRetiro string, status string) (*pb.Order, error) {
	filter := bson.M{"qr_retiro": qrRetiro}
	update := bson.M{"$set": bson.M{"status": status}}

	var updatedOrder pb.Order
	err := db.collection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&updatedOrder)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &updatedOrder, nil
}
