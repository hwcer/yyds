package bson

import "go.mongodb.org/mongo-driver/bson"

const FieldUpdateSetOnInsert = "$setOnInsert"

var fieldUpdateHandler = map[string]func(document Document, m bson.M) error{}

var fieldUpdateOperators = []string{"$inc", "$min", "max", "$mul", "$set"}

var fieldUpdateSetOnInsert = append([]string{FieldUpdateSetOnInsert}, fieldUpdateOperators...)
