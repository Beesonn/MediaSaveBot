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
)

type User struct {
    UserID    int64     `bson:"user_id"`
    Name      string    `bson:"name"`
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

func SaveUser(ctx context.Context, name string, usrid string) {
    if mongoClient == nil {
        if err := InitDB(); err != nil {
            log.Printf("failed to initialize database: %v", err)
        }
    }
    
    dbUser := &User{
        UserID:   usrid,
        Name:     name,
    }   
    if chk, _ := GetUser(context.Background(), usrid); chk == nil {
        _, err := userCollection.InsertOne(ctx, dbUser)
        if err != nil {
            log.Printf("error saving user: %v", err)
        }
        log.Printf("User %d saved successfully", user.UserID)
    }
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
            return nil, nil
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

func GetUserCount(ctx context.Context) (int64, error) {
    if mongoClient == nil {
        if err := InitDB(); err != nil {
            return 0, fmt.Errorf("failed to initialize database: %v", err)
        }
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
