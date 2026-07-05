package repository

import (
	"context"
	"errors"
	"log"
	"time"

	pb "foodrush/orders/proto"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	if order.Outbox == nil {
		order.Outbox = []*pb.OutboxEvent{}
	}
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

func (db *MongoDB) UpdateOrderStatusByID(ctx context.Context, orderID string, status string) error {
	filter := bson.M{"id": orderID}
	update := bson.M{
		"$set": bson.M{
			"status": status,
		},
	}

	result, err := db.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func (db *MongoDB) GetUnprocessedEvents(ctx context.Context) ([]*pb.Order, error) {
	// Buscamos órdenes que tengan al menos un evento en la outbox con processed: false
	filter := bson.M{"outbox": bson.M{"$elemMatch": bson.M{"processed": false}}}
	cursor, err := db.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []*pb.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	log.Printf("[orders-service] DB: GetUnprocessedEvents found %d orders", len(orders))
	return orders, nil
}

func (db *MongoDB) MarkEventAsProcessed(ctx context.Context, orderID string, eventID string) error {
	filter := bson.M{"id": orderID, "outbox.id": eventID}
	update := bson.M{"$set": bson.M{"outbox.$.processed": true}}
	res, err := db.collection.UpdateOne(ctx, filter, update)
	if err == nil && res.MatchedCount == 0 {
		log.Printf("[orders-service] MarkEventAsProcessed: no document matched orderID=%s eventID=%s", orderID, eventID)
	}
	return err
}

func (db *MongoDB) AddOutboxEvent(ctx context.Context, orderID string, event *pb.OutboxEvent) error {
	filter := bson.M{"id": orderID}
	update := bson.M{"$push": bson.M{"outbox": event}}
	_, err := db.collection.UpdateOne(ctx, filter, update)
	return err
}
