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
	mongoClient            *mongo.Client
	userCollection         *mongo.Collection
	cloneBotCollection     *mongo.Collection
	cloneBotUserCollection *mongo.Collection
	mongoAvailable         = false
)

type User struct {
	UserID int64  `bson:"user_id"`
	Name   string `bson:"name"`
}

type CloneBot struct {
	BotID     int64     `bson:"bot_id"`
	BotToken  string    `bson:"bot_token"`
	OwnerID   int64     `bson:"owner_id"`
	Username  string    `bson:"username"`
	CreatedAt time.Time `bson:"created_at"`
}

type CloneBotUser struct {
	BotID     int64     `bson:"bot_id"`
	UserID    int64     `bson:"user_id"`
	Name      string    `bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
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
	cloneBotCollection = db.Collection("clone_bots")
	cloneBotUserCollection = db.Collection("clone_bot_users")
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
		}
	}
}

func SaveCloneBot(botID, ownerID int64, username, botToken string) {
	if !mongoAvailable {
		return
	}

	existing, _ := GetCloneBotByID(botID)
	if existing != nil {
		filter := bson.M{"bot_id": botID}
		update := bson.M{"$set": bson.M{
			"bot_token": botToken,
			"username":  username,
		}}
		_, err := cloneBotCollection.UpdateOne(context.Background(), filter, update)
		if err != nil {
			log.Printf("error updating clone bot: %v", err)
		}
		return
	}

	cloneBot := &CloneBot{
		BotID:     botID,
		BotToken:  botToken,
		OwnerID:   ownerID,
		Username:  username,
		CreatedAt: time.Now(),
	}

	_, err := cloneBotCollection.InsertOne(context.Background(), cloneBot)
	if err != nil {
		log.Printf("error saving clone bot: %v", err)
	}
}

func SaveCloneBotUser(botID, userID int64, name string) {
	if !mongoAvailable {
		return
	}

	existing := GetCloneBotUser(botID, userID)
	if existing != nil {
		return
	}

	cloneBotUser := &CloneBotUser{
		BotID:     botID,
		UserID:    userID,
		Name:      name,
		CreatedAt: time.Now(),
	}

	_, err := cloneBotUserCollection.InsertOne(context.Background(), cloneBotUser)
	if err != nil {
		log.Printf("error saving clone bot user: %v", err)
	}
}

func GetCloneBotByID(botID int64) (*CloneBot, error) {
	if !mongoAvailable {
		return nil, nil
	}

	var bot CloneBot
	filter := bson.M{"bot_id": botID}
	err := cloneBotCollection.FindOne(context.Background(), filter).Decode(&bot)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

func GetCloneBotUser(botID, userID int64) *CloneBotUser {
	if !mongoAvailable {
		return nil
	}

	var user CloneBotUser
	filter := bson.M{"bot_id": botID, "user_id": userID}
	err := cloneBotUserCollection.FindOne(context.Background(), filter).Decode(&user)

	if err != nil {
		return nil
	}
	return &user
}

func GetCloneBotUsers(botID int64) ([]CloneBotUser, error) {
	if !mongoAvailable {
		return []CloneBotUser{}, nil
	}

	cursor, err := cloneBotUserCollection.Find(context.Background(), bson.M{"bot_id": botID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var users []CloneBotUser
	for cursor.Next(context.Background()) {
		var user CloneBotUser
		err := cursor.Decode(&user)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func GetAllCloneBotsUsersCount() int64 {
	if !mongoAvailable {
		return 0
	}

	count, err := cloneBotUserCollection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Printf("error counting clone bot users: %v", err)
		return 0
	}
	return count
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

	return users, nil
}

func GetAllCloneBots(ctx context.Context) ([]CloneBot, error) {
	if !mongoAvailable {
		return []CloneBot{}, nil
	}

	cursor, err := cloneBotCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error getting all clone bots: %v", err)
	}
	defer cursor.Close(ctx)

	var bots []CloneBot
	for cursor.Next(ctx) {
		var bot CloneBot
		err := cursor.Decode(&bot)
		if err != nil {
			return nil, fmt.Errorf("error decoding clone bot: %v", err)
		}
		bots = append(bots, bot)
	}

	return bots, nil
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

func GetCloneBotCount(ctx context.Context) (int64, error) {
	if !mongoAvailable {
		return 0, nil
	}

	count, err := cloneBotCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("error counting clone bots: %v", err)
	}
	return count, nil
}

func DeleteCloneBotByID(ctx context.Context, botID int64) error {
	if !mongoAvailable {
		return nil
	}

	filter := bson.M{"bot_id": botID}
	_, err := cloneBotCollection.DeleteOne(ctx, filter)
	if err != nil {
		log.Printf("error deleting clone bot %d: %v", botID, err)
		return err
	}

	_, err = cloneBotUserCollection.DeleteMany(ctx, bson.M{"bot_id": botID})
	if err != nil {
		log.Printf("error deleting clone bot users %d: %v", botID, err)
	}

	log.Printf("Clone bot %d removed from database", botID)
	return nil
}

func CloseDB() error {
	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return mongoClient.Disconnect(ctx)
	}
	return nil
}
