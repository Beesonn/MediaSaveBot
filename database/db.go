package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoClient    *mongo.Client
	userCollection *mongo.Collection
	mongoAvailable = false
)

type User struct {
	UserID int64  `bson:"user_id"`
	Name   string `bson:"name"`
}

func InitDB() error {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Println("MONGODB_URI not set, running without database")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("Error connecting to MongoDB: %v, running without database", err)
		return nil
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Error pinging MongoDB: %v, running without database", err)
		return nil
	}

	mongoClient = client
	db := client.Database("media_save_bot")
	userCollection = db.Collection("users")
	mongoAvailable = true

	log.Println("MongoDB initialized successfully")
	return nil
}

func IsMongoAvailable() bool {
	return mongoAvailable
}

func SaveUser(ctx context.Context, name string, usrid int64) {
	if !mongoAvailable {
		return
	}

	dbUser := &User{
		UserID: usrid,
		Name:   name,
	}

	if chk, _ := GetUser(context.Background(), usrid); chk == nil {
		_, err := userCollection.InsertOne(ctx, dbUser)
		if err != nil {
			log.Printf("error saving user: %v", err)
		} else {
			log.Printf("User %d saved successfully", usrid)
		}
	}
}

func GetUser(ctx context.Context, userID int64) (*User, error) {
	if !mongoAvailable {
		return nil, nil
	}

	var user User
	filter := bson.M{"user_id": userID}
	err := userCollection.FindOne(ctx, filter).Decode(&user)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting user: %v", err)
	}
	return &user, nil
}

func GetAllUsers(ctx context.Context) ([]User, error) {
	if !mongoAvailable {
		return []User{}, nil
	}

	cursor, err := userCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error getting all users: %v", err)
	}
	defer cursor.Close(ctx)

	var users []User
	for cursor.Next(ctx) {
		var user User
		err := cursor.Decode(&user)
		if err != nil {
			return nil, fmt.Errorf("error decoding user: %v", err)
		}
		users = append(users, user)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %v", err)
	}
	return users, nil
}

func GetUserCount(ctx context.Context) (int64, error) {
	if !mongoAvailable {
		return 0, nil
	}

	count, err := userCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error counting users: %v", err)
	}
	return count, nil
}

func CloseDB() error {
	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return mongoClient.Disconnect(ctx)
	}
	return nil
}
