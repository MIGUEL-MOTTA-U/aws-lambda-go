package model

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type ListingID string

func (id *ListingID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = ListingID(s)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*id = ListingID(fmt.Sprintf("%.0f", n))
		return nil
	}
	return fmt.Errorf("listing_id must be a string or number")
}

func (id ListingID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id))
}

func (id ListingID) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberS{Value: string(id)}, nil
}

func (id *ListingID) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	if av == nil {
		return nil
	}
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		*id = ListingID(v.Value)
		return nil
	case *types.AttributeValueMemberN:
		*id = ListingID(v.Value)
		return nil
	default:
		return fmt.Errorf("listing_id attribute must be a string or number")
	}
}

type Coordinates struct {
	Lat float64 `json:"lat" dynamodbav:"lat"`
	Lng float64 `json:"lng" dynamodbav:"lng"`
}

type Location struct {
	Country      string      `json:"country" dynamodbav:"country"`
	State        string      `json:"state" dynamodbav:"state"`
	City         string      `json:"city" dynamodbav:"city"`
	Neighborhood string      `json:"neighborhood" dynamodbav:"neighborhood"`
	Address      string      `json:"address" dynamodbav:"address"`
	Stratum      int         `json:"stratum" dynamodbav:"stratum"`
	Coordinates  Coordinates `json:"coordinates" dynamodbav:"coordinates"`
}

type Pricing struct {
	SalePrice        float64 `json:"sale_price" dynamodbav:"sale_price"`
	RentPrice        float64 `json:"rent_price" dynamodbav:"rent_price"`
	AdminFee         float64 `json:"admin_fee" dynamodbav:"admin_fee"`
	Taxes            float64 `json:"taxes" dynamodbav:"taxes"`
	Currency         string  `json:"currency" dynamodbav:"currency"`
	DisplayPriceText string  `json:"display_price_text" dynamodbav:"display_price_text"`
}

type Areas struct {
	LandAreaM2    float64 `json:"land_area_m2" dynamodbav:"land_area_m2"`
	BuiltAreaM2   float64 `json:"built_area_m2" dynamodbav:"built_area_m2"`
	PrivateAreaM2 float64 `json:"private_area_m2" dynamodbav:"private_area_m2"`
	LotAreaM2     float64 `json:"lot_area_m2" dynamodbav:"lot_area_m2"`
	FrontM        float64 `json:"front_m" dynamodbav:"front_m"`
	BackM         float64 `json:"back_m" dynamodbav:"back_m"`
}

type Layout struct {
	Bedrooms      int `json:"bedrooms" dynamodbav:"bedrooms"`
	Bathrooms     int `json:"bathrooms" dynamodbav:"bathrooms"`
	HalfBathrooms int `json:"half_bathrooms" dynamodbav:"half_bathrooms"`
	ParkingSpaces int `json:"parking_spaces" dynamodbav:"parking_spaces"`
	Floors        int `json:"floors" dynamodbav:"floors"`
	UnitFloor     int `json:"unit_floor" dynamodbav:"unit_floor"`
}

type Structure struct {
	YearBuilt           int    `json:"year_built" dynamodbav:"year_built"`
	AgeYears            int    `json:"age_years" dynamodbav:"age_years"`
	ConstructionQuality string `json:"construction_quality" dynamodbav:"construction_quality"`
	ConservationStatus  string `json:"conservation_status" dynamodbav:"conservation_status"`
	TerrainType         string `json:"terrain_type" dynamodbav:"terrain_type"`
	StructureType       string `json:"structure_type" dynamodbav:"structure_type"`
	BuiltLevels         int    `json:"built_levels" dynamodbav:"built_levels"`
}

type Features struct {
	Indoor     []string `json:"indoor" dynamodbav:"indoor"`
	Outdoor    []string `json:"outdoor" dynamodbav:"outdoor"`
	Commercial []string `json:"commercial" dynamodbav:"commercial"`
	Project    []string `json:"project" dynamodbav:"project"`
}

type Media struct {
	Photos            []string `json:"photos" dynamodbav:"photos"`
	PhotoCount        int      `json:"photo_count" dynamodbav:"photo_count"`
	HasMap            bool     `json:"has_map" dynamodbav:"has_map"`
	HasVideo          bool     `json:"has_video" dynamodbav:"has_video"`
	HasFloorplans     bool     `json:"has_floorplans" dynamodbav:"has_floorplans"`
	HasVirtualTour360 bool     `json:"has_virtual_tour_360" dynamodbav:"has_virtual_tour_360"`
}

type Commercial struct {
	AgentName    string `json:"agent_name" dynamodbav:"agent_name"`
	OfficeName   string `json:"office_name" dynamodbav:"office_name"`
	Phone        string `json:"phone" dynamodbav:"phone"`
	Email        string `json:"email" dynamodbav:"email"`
	WhatsappLink string `json:"whatsapp_link" dynamodbav:"whatsapp_link"`
	OfficeHours  string `json:"office_hours" dynamodbav:"office_hours"`
}

type ListingMetadata struct {
	UpdatedAt      string   `json:"updated_at" dynamodbav:"updated_at"`
	UpdatedAgeText string   `json:"updated_age_text" dynamodbav:"updated_age_text"`
	Breadcrumbs    []string `json:"breadcrumbs" dynamodbav:"breadcrumbs"`
	SourceSystem   string   `json:"source_system" dynamodbav:"source_system"`
}

type Listing struct {
	ListingID         ListingID       `json:"listing_id" dynamodbav:"listing_id"`
	Slug              string          `json:"slug" dynamodbav:"slug"`
	URL               string          `json:"url" dynamodbav:"url"`
	Language          string          `json:"language" dynamodbav:"language"`
	Title             string          `json:"title" dynamodbav:"title"`
	PropertyType      string          `json:"property_type" dynamodbav:"property_type"`
	Subtype           string          `json:"subtype" dynamodbav:"subtype"`
	OperationType     string          `json:"operation_type" dynamodbav:"operation_type"`
	PublicationStatus string          `json:"publication_status" dynamodbav:"publication_status"`
	Location          Location        `json:"location" dynamodbav:"location"`
	Pricing           Pricing         `json:"pricing" dynamodbav:"pricing"`
	Areas             Areas           `json:"areas" dynamodbav:"areas"`
	Layout            Layout          `json:"layout" dynamodbav:"layout"`
	Structure         Structure       `json:"structure" dynamodbav:"structure"`
	Features          Features        `json:"features" dynamodbav:"features"`
	Media             Media           `json:"media" dynamodbav:"media"`
	Commercial        Commercial      `json:"commercial" dynamodbav:"commercial"`
	Metadata          ListingMetadata `json:"metadata" dynamodbav:"metadata"`
}
