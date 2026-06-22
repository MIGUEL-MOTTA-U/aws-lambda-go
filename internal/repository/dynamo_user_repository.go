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

type DynamoDBAPI interface {
	DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

type DynamoUserRepository struct {
	db        DynamoDBAPI
	tableName string
}

func NewDynamoUserRepository(db DynamoDBAPI, tableName string) *DynamoUserRepository {
	return &DynamoUserRepository{
		db:        db,
		tableName: tableName,
	}
}

func (r DynamoUserRepository) FindAll(ctx context.Context) ([]model.User, error) {
	users := make([]model.User, 0)
	var startKey map[string]types.AttributeValue

	for {
		out, err := r.db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(r.tableName),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}

		var page []model.User
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
			return nil, err
		}

		users = append(users, page...)
		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	return users, nil
}

func (r DynamoUserRepository) FindByID(ctx context.Context, id string) (model.User, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       userKey(id),
	})
	if err != nil {
		return model.User{}, err
	}
	if len(out.Item) == 0 {
		return model.User{}, ErrUserNotFound
	}

	var user model.User
	if err := attributevalue.UnmarshalMap(out.Item, &user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

func (r DynamoUserRepository) Create(ctx context.Context, user model.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	if err != nil {
		var conditionalErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalErr) {
			return ErrUserAlreadyExists
		}
		return err
	}

	return nil
}

func (r DynamoUserRepository) Update(ctx context.Context, user model.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r DynamoUserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:           aws.String(r.tableName),
		Key:                 userKey(id),
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		var conditionalErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalErr) {
			return ErrUserNotFound
		}
		return err
	}

	return nil
}

func userKey(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: id},
	}
}
