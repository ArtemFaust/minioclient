package topicoperation

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/minio/minio-go/v7"
)

// параметры топика
type TopicConfig struct {
	TopicName         string // Имя топика
	PushEndpoint      string // Строка определяет точку назначения отправки уведомлений https://endpoint.com
	UseSsl            bool   // Определяет, проверять ли SSL-сертификат,
	VerifySsl         bool   // Нужно ли строго валидировать цепочку сертификатов вебхука
	HttpAuthUser      string // Имя пользователя базовой аутентификации
	HttpAuthPassword  string // Пароль пользователя базовой аутентификации
	CloudeventsFormat bool   // Включить или отключить формар CloudEvents (по умолчанию json)
}

// Методы управления топиком
// Создание топика - если топик существует он будет обновлен
func CreateOrUpdateTopic(ctx context.Context, minioClient *minio.Client, config TopicConfig) (string, error) {
	snsClient, e := createSnsClient(ctx, minioClient)
	if e != nil {
		return "", e
	}

	// Отправляем запрос на создание топика
	result, e := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(config.TopicName),
		Attributes: map[string]string{
			"push-endpoint":      config.PushEndpoint,
			"use-ssl":            strconv.FormatBool(config.UseSsl),
			"verify-ssl":         strconv.FormatBool(config.VerifySsl),
			"http-auth-user":     config.HttpAuthUser,
			"http-auth-password": config.HttpAuthPassword,
			"cloudevents-format": strconv.FormatBool(config.CloudeventsFormat),
		},
	})
	if e != nil {
		return "", e
	}

	return *result.TopicArn, nil
}

// Получить список топиков
func GetTopicsList(ctx context.Context, minioClient *minio.Client) ([]string, error) {
	var allTopics []string
	snsClient, e := createSnsClient(ctx, minioClient)
	if e != nil {
		return allTopics, e
	}
	// Инициализируем пагинатор
	paginator := sns.NewListTopicsPaginator(snsClient, &sns.ListTopicsInput{})

	// Цикл идет по страницам (каждая страница отдает до 100 топиков)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения страницы топиков: %w", err)
		}

		// Собираем топики с текущей страницы
		for _, topic := range page.Topics {
			if topic.TopicArn != nil {
				allTopics = append(allTopics, *topic.TopicArn)
			}
		}
	}

	return allTopics, nil
}

// Метод получения информации о топике
func GetTopiCInfo(ctx context.Context, minioClient *minio.Client, arn string) (*sns.GetTopicAttributesOutput, error) {
	var attributes *sns.GetTopicAttributesOutput
	snsClient, e := createSnsClient(ctx, minioClient)
	if e != nil {
		return attributes, e
	}
	attributes, e = snsClient.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: &arn,
	})
	if e != nil {
		return attributes, e
	}
	return attributes, nil
}

// Метод удаления топика
func RemoveTopic(ctx context.Context, minioClient *minio.Client, arn string) (*sns.DeleteTopicOutput, error) {
	var o *sns.DeleteTopicOutput
	snsClient, e := createSnsClient(ctx, minioClient)
	if e != nil {
		return o, e
	}
	o, e = snsClient.DeleteTopic(ctx, &sns.DeleteTopicInput{
		TopicArn: &arn,
	})
	if e != nil {
		return o, e
	}
	return o, nil
}

// Метод создания SNS клиента
func createSnsClient(ctx context.Context, minioClient *minio.Client) (*sns.Client, error) {
	if minioClient == nil {
		return nil, fmt.Errorf("Ошибка - не установленно подключение")
	}
	//  Вытаскиваем credentials и endpoint напрямую из готового minioClient
	credsValue, e := minioClient.GetCreds()
	if e != nil {
		return nil, fmt.Errorf("не удалось получить ключи из minioClient: %w", e)
	}

	// Подготавливаем url подключения
	rgwEndpoint := minioClient.EndpointURL().Scheme + "://" + minioClient.EndpointURL().Host

	// Инициализируем конфиг AWS SDK, используя данные из MinIO
	cfg, e := config.LoadDefaultConfig(ctx,
		config.WithRegion("default"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			credsValue.AccessKeyID,
			credsValue.SecretAccessKey,
			credsValue.SessionToken,
		)),
	)
	if e != nil {
		return nil, e
	}

	// Создаем SNS-клиент, переиспользуя endpoint
	snsClient := sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String(rgwEndpoint)
	})
	return snsClient, nil
}
