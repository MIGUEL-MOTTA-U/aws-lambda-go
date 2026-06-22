package repository

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"rs-lambda-go/internal/model"
)

type DynamoListingRepository struct {
	db        DynamoDBAPI
	tableName string
}

func NewDynamoListingRepository(db DynamoDBAPI, tableName string) *DynamoListingRepository {
	return &DynamoListingRepository{
		db:        db,
		tableName: tableName,
	}
}

func (r DynamoListingRepository) FindAll(ctx context.Context) ([]model.Listing, error) {
	listings := make([]model.Listing, 0)
	var startKey map[string]types.AttributeValue

	for {
		out, err := r.db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(r.tableName),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}

		var page []model.Listing
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
			return nil, err
		}

		listings = append(listings, page...)
		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	return listings, nil
}

func (r DynamoListingRepository) FindByID(ctx context.Context, id string) (model.Listing, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       listingKey(id),
	})
	if err != nil {
		return model.Listing{}, err
	}
	if len(out.Item) == 0 {
		return model.Listing{}, ErrListingNotFound
	}

	var listing model.Listing
	if err := attributevalue.UnmarshalMap(out.Item, &listing); err != nil {
		return model.Listing{}, err
	}

	return listing, nil
}

func (r DynamoListingRepository) Create(ctx context.Context, listing model.Listing) error {
	item, err := attributevalue.MarshalMap(listing)
	if err != nil {
		return err
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(listing_id)"),
	})
	if err != nil {
		var conditionalErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalErr) {
			return ErrListingAlreadyExists
		}
		return err
	}

	return nil
}

func (r DynamoListingRepository) Update(ctx context.Context, listing model.Listing) error {
	item, err := attributevalue.MarshalMap(listing)
	if err != nil {
		return err
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r DynamoListingRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:           aws.String(r.tableName),
		Key:                 listingKey(id),
		ConditionExpression: aws.String("attribute_exists(listing_id)"),
	})
	if err != nil {
		var conditionalErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalErr) {
			return ErrListingNotFound
		}
		return err
	}

	return nil
}

func listingKey(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"listing_id": &types.AttributeValueMemberS{Value: id},
	}
}
