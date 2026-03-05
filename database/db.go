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
    mongoClient *mongo.Client
    userCollection *mongo.Collection
)

type User struct {
    UserID    int64     `bson:"user_id"`
    FirstName string    `bson:"first_name"`
    LastName  string    `bson:"last_name"`
    Username  string    `bson:"username"`
    ChatID    int64     `bson:"chat_id"`
    JoinedAt  time.Time `bson:"joined_at"`
}

func InitDB() error {
    mongoURI := os.Getenv("MONGODB_URI")
    if mongoURI == "" {
        return fmt.Errorf("MONGODB_URI environment variable is empty")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    clientOptions := options.Client().ApplyURI(mongoURI)

    client, err := mongo.Connect(ctx, clientOptions)
    if err != nil {
        return fmt.Errorf("error connecting to MongoDB: %v", err)
    }

    err = client.Ping(ctx, nil)
    if err != nil {
        return fmt.Errorf("error pinging MongoDB: %v", err)
    }

    mongoClient = client
    db := client.Database("media_save_bot")
    userCollection = db.Collection("users")

    log.Println("MongoDB initialized successfully")
    return nil
}

func SaveUser(ctx context.Context, user *User) error {
    if mongoClient == nil {
        if err := InitDB(); err != nil {
            return fmt.Errorf("failed to initialize database: %v", err)
        }
    }

    user.JoinedAt = time.Now()

    filter := bson.M{"user_id": user.UserID}
    update := bson.M{"$set": user}
    opts := options.Update().SetUpsert(true)

    _, err := userCollection.UpdateOne(ctx, filter, update, opts)
    if err != nil {
        return fmt.Errorf("error saving user: %v", err)
    }

    log.Printf("User %d saved successfully", user.UserID)
    return nil
}

func GetUser(ctx context.Context, userID int64) (*User, error) {
    if mongoClient == nil {
        if err := InitDB(); err != nil {
            return nil, fmt.Errorf("failed to initialize database: %v", err)
        }
    }

    var user User
    filter := bson.M{"user_id": userID}
    err := userCollection.FindOne(ctx, filter).Decode(&user)

    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, nil // User not found
        }
        return nil, fmt.Errorf("error getting user: %v", err)
    }

    return &user, nil
}

func GetAllUsers(ctx context.Context) ([]User, error) {
    if mongoClient == nil {
        if err := InitDB(); err != nil {
            return nil, fmt.Errorf("failed to initialize database: %v", err)
        }
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

func CloseDB() error {
    if mongoClient != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        return mongoClient.Disconnect(ctx)
    }
    return nil
}
