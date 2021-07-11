package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/golang/snappy"
	"google.golang.org/api/option"
)

func create_client(GCPKey string) *storage.Client {
	ctx := context.Background()

	// jsonで渡された鍵のサービスアカウントに紐づけられたクライアントを建てる
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(GCPKey))
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func create_bucket(client storage.Client, projectID string) (*storage.BucketHandle, string) {
	// "s512_local" + バックアップ日時 をバケット名にする
	t := time.Now()
	bucketName := fmt.Sprintf("s512_local-%d-%d-%d", t.Year(), t.Month(), t.Day())

	// バケットとメタデータの設定
	ctx := context.Background()
	bucket := client.Bucket(bucketName)
	bucketAtters := &storage.BucketAttrs{
		StorageClass: "COLDLINE",
		Location:     "asia-northeast1",
		// 生成から90日でバケットを削除
		Lifecycle: storage.Lifecycle{Rules: []storage.LifecycleRule{
			{
				Action:    storage.LifecycleAction{Type: "Delete"},
				Condition: storage.LifecycleCondition{AgeInDays: 90},
			},
		}},
	}

	// バケットの作成
	err := bucket.Create(ctx, projectID, bucketAtters)
	if err != nil {
		log.Fatal(err)
	}

	return bucket, fmt.Sprintf("Bucket \"%s\" successfully created", bucketName)
}

func copy_directory(bucket storage.BucketHandle, localPath string) string {
	// ローカルのディレクトリ構造を読み込み
	bu_files, err := ioutil.ReadDir(localPath)
	if err != nil {
		log.Fatal(err)
	}

	// ファイルを1つずつストレージにコピー
	for _, file := range bu_files {
		copy_file(bucket, file, localPath)
		log.Println("Copied", file.Name())
	}

	return fmt.Sprintf("%d file(s) successfully copied", len(bu_files))
}

func copy_file(bucket storage.BucketHandle, file fs.FileInfo, localPath string) {
	// ローカルのファイルを開く
	original, err := os.Open(localPath + "/" + file.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer original.Close()

	//書き込むためのWriterを作成
	ctx := context.Background()
	writer := bucket.Object(file.Name()).NewWriter(ctx)
	snappyWriter := snappy.NewBufferedWriter(writer)
	defer snappyWriter.Close()
	defer writer.Close()

	// 書きこみ
	_, err = io.Copy(snappyWriter, original)
	if err != nil {
		log.Fatal(err)
	}
}
