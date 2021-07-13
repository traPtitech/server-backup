package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/traPtitech/localfile-backup-helper/gcp"
	"github.com/traPtitech/localfile-backup-helper/webhook"
)

const timeFormat = "2006/01/02 15:04:05"

// パッケージを表す構造体の定義
type MainStruct struct {
	LocalPath  string
	ProjectId  string
	BucketName string
}

// 各パッケージを表す構造体型の変数の定義
var (
	mainStruct    MainStruct
	gcpStruct     gcp.GcpStruct
	webhookStruct webhook.WebhookStruct
)

func init() {
	// 環境変数を取得
	localPath := os.Getenv("LOCAL_PATH")
	gcpKey := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	projectId := os.Getenv("PROJECT_ID")
	bucketName := os.Getenv("BUCKET_NAME")
	storageClass := os.Getenv("STORAGECLASS")
	duration, _ := strconv.ParseInt(os.Getenv("DURATION"), 0, 64)
	webhookId := os.Getenv("TRAQ_WEBHOOK_ID")
	webhookSecret := os.Getenv("TRAQ_WEBHOOK_SECRET")

	// 環境変数がどれか一つでも空だったらエラーを吐いて終了
	if localPath == "" || gcpKey == "" || projectId == "" || bucketName == "" || storageClass == "" || duration == 0 || webhookId == "" || webhookSecret == "" {
		log.Print("Error: Failed to load env-vars")
		panic("empty env-var(s) exist")
	}

	// 各パッケージを表す構造体型の変数に、取得した環境変数を代入
	mainStruct = MainStruct{
		LocalPath:  localPath,
		ProjectId:  projectId,
		BucketName: bucketName,
	}
	gcpStruct = gcp.GcpStruct{
		LocalPath:    localPath,
		GcpKey:       gcpKey,
		ProjectId:    projectId,
		StorageClass: storageClass,
		Duration:     duration,
	}
	webhookStruct = webhook.WebhookStruct{
		WebhookId:     webhookId,
		WebhookSecret: webhookSecret,
	}
}

func main() {
	log.Print("Backin' up files from", mainStruct.LocalPath, "to", mainStruct.ProjectId, "on gcp Storage...")
	startTime := time.Now()

	// クライアントを建てる
	client, err := gcpStruct.CreateClient()
	if err != nil {
		log.Print("Error: Failed to load create client")
		panic(err)
	}
	defer client.Close()

	// bucketName + バックアップ日 をバケット名とする
	t := &startTime
	bucketName := fmt.Sprintf("%s-%d-%d-%d", mainStruct.BucketName, t.Year(), t.Month(), t.Day())

	// バケットを作成
	bucket, err := gcpStruct.CreateBucket(*client, bucketName)
	if err != nil {
		log.Print("Error: Failed to create bucket")
		panic(err)
	}

	// バケットへファイルをコピー
	objectNum, err, errs := gcpStruct.CopyDirectory(*bucket)
	if err != nil {
		log.Print("Error: Failed to load local directory")
		panic(err)
	}
	log.Printf("%d file(s) successfully copied, %d error(s) occured", objectNum, len(errs))
	if len(errs) != 0 {
		for i, err := range errs {
			log.Printf("Error %d: %s", i, err)
		}
	}

	// Webhook用のメッセージを作成
	endTime := time.Now()
	buDuration := endTime.Sub(startTime)
	mes := mainStruct.CreateMes(bucketName, startTime, buDuration, objectNum, len(errs))

	// WebhookをtraQ Webhook Botに送信
	err = webhookStruct.SendWebhook(mes)
	if err != nil {
		log.Print("Failed to send webhook")
		panic(err)
	}
}

func (env *MainStruct) CreateMes(bucketName string, startTime time.Time, buDuration time.Duration, objectNum int, errs_num int) string {
	// traQに流すテキストメッセージを生成
	mes := fmt.Sprintf(
		`### ローカルファイルのバックアップが保存されました
	バックアップ元ディレクトリ: %s 
	生成されたバケット名: %s
	バックアップ開始時刻: %s
	バックアップ所要時間: %f 分
	オブジェクト数: %d
	エラー数: %d`,
		env.LocalPath, bucketName, startTime.Format(timeFormat), buDuration.Minutes(), objectNum, errs_num)

	log.Print("Webhook message generated")
	return mes
}
