package model

type User struct {
	ID           string `json:"id" gorm:"primaryKey;column:id"`
	Name         string `json:"name" gorm:"column:name"`
	Email        string `json:"email" gorm:"column:email"`
	Username     string `json:"username" gorm:"column:username"`
	Birthdate    string `json:"birthdate" gorm:"column:birthdate"`
	CreationDate string `json:"creationdate" gorm:"column:creationdate"`
}
