package repository

import (
	"context"
	"errors"
	"time"

	"github.com/foodrush/observability"
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
	dbCtx, span := observability.StartDBSpan(ctx, "mongodb", "insertOne orders")
	defer span.End()

	_, err := db.collection.InsertOne(dbCtx, order)
	return err
}

func (db *MongoDB) GetOrder(ctx context.Context, id string) (*pb.Order, error) {
	dbCtx, span := observability.StartDBSpan(ctx, "mongodb", "findOne orders")
	defer span.End()

	var order pb.Order
	err := db.collection.FindOne(dbCtx, bson.M{"id": id}).Decode(&order)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (db *MongoDB) UpdateOrderStatus(ctx context.Context, qrRetiro string, status string) (*pb.Order, error) {
	dbCtx, span := observability.StartDBSpan(ctx, "mongodb", "findOneAndUpdate orders")
	defer span.End()

	filter := bson.M{"qr_retiro": qrRetiro}
	update := bson.M{"$set": bson.M{"status": status}}

	var updatedOrder pb.Order
	err := db.collection.FindOneAndUpdate(dbCtx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&updatedOrder)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &updatedOrder, nil
}
