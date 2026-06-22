package model

type User struct {
	ID           string `json:"id" dynamodbav:"id"`
	Name         string `json:"name" dynamodbav:"name"`
	Email        string `json:"email" dynamodbav:"email"`
	Username     string `json:"username" dynamodbav:"username"`
	Birthdate    string `json:"birthdate" dynamodbav:"birthdate"`
	CreationDate string `json:"creationdate" dynamodbav:"creationdate"`
}
