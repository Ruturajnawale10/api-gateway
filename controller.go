package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
)

type User struct {
	UserName string `bson:"username"`
	Password string `bson:"password"`
}

var userCollection = db().Database("r_tpb_1").Collection("user")

func signInUser(w http.ResponseWriter, r *http.Request) {
	fmt.Println("signInUser API called")
	w.Header().Set("Content-Type", "application/json")

	var user User
	err := json.NewDecoder(r.Body).Decode(&user)

	if err != nil {
		log.Fatal(err)
	}

	filter := bson.M{"username": user.UserName, "password": user.Password}

	var result User
	err = userCollection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode("Invalid Credentials")
		return
	}

	fmt.Println("Found a record: ", result)
	json.NewEncoder(w).Encode(http.StatusOK)
}
