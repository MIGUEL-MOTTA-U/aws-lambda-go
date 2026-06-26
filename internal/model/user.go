package model

type UserMetadata struct {
	Stats        interface{} `json:"stats,omitempty"`
	Badges       interface{} `json:"badges,omitempty"`
	Services     interface{} `json:"services,omitempty"`
	HeroImageURL string      `json:"hero_image_url,omitempty"`
	HeroVideoURL string      `json:"hero_video_url,omitempty"`
}

type User struct {
	ID            string       `json:"id" gorm:"primaryKey;column:id"`
	Name          string       `json:"name" gorm:"column:name"`
	Email         string       `json:"email" gorm:"column:email"`
	Username      string       `json:"username" gorm:"column:username"`
	Birthdate     string       `json:"birthdate" gorm:"column:birthdate"`
	CreationDate  string       `json:"creationdate" gorm:"column:creationdate"`
	Phone         string       `json:"phone,omitempty" gorm:"column:phone"`
	Role          string       `json:"role,omitempty" gorm:"column:role"`
	Company       string       `json:"company,omitempty" gorm:"column:company"`
	OfficeName    string       `json:"office_name,omitempty" gorm:"column:office_name"`
	OfficeAddress string       `json:"office_address,omitempty" gorm:"column:office_address"`
	License       string       `json:"license,omitempty" gorm:"column:license"`
	Bio           string       `json:"bio,omitempty" gorm:"column:bio"`
	Headline      string       `json:"headline,omitempty" gorm:"column:headline"`
	AvatarURL     string       `json:"avatar_url,omitempty" gorm:"column:avatar_url"`
	AvatarAssetID string       `json:"avatar_asset_id,omitempty" gorm:"column:avatar_asset_id"`
	WhatsAppLink  string       `json:"whatsapp_link,omitempty" gorm:"column:whatsapp_link"`
	InstagramURL  string       `json:"instagram_url,omitempty" gorm:"column:instagram_url"`
	LinkedInURL   string       `json:"linkedin_url,omitempty" gorm:"column:linkedin_url"`
	FacebookURL   string       `json:"facebook_url,omitempty" gorm:"column:facebook_url"`
	Metadata      UserMetadata `json:"metadata,omitempty" gorm:"column:metadata"`
}
