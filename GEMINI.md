# Gemini コンテキスト: Goアーキテクチャワークショップ

## プロジェクト概要

このリポジトリは、Google Cloud上でのシステムアーキテクチャ設計ワークショップとして作られた、Goベースのアプリケーションを含んでいます。このプロジェクトは、ユーザーとアイテムを管理するシンプルなゲーム関連API (`game-api`) を実装しています。

**主要技術:**
- **言語:** Go
- **APIフレームワーク:** 標準ライブラリ `net/http` (`cmd/api/main.go` から推測)
- **データベース:** Google Cloud Spanner
- **キャッシュ:** Redis (Google Cloud Memorystore経由)
- **デプロイ:** Google Cloud Run
- **Infrastructure as Code:** Terraform
- **コンテナ化:** Docker と Docker Compose (ローカル開発用)

**アーキテクチャ:**
このアプリケーションは、永続ストレージとしてCloud Spannerデータベースに、キャッシュとしてRedisインスタンスに接続するモノリシックなAPIサービス (`game-api`) です。Cloud Run上でサーバーレスコンテナとしてデプロイされるように設計されています。Google Cloud上で必要となるインフラ（VPC, Spanner, Memorystore, Cloud Runサービスなど）はTerraformを使って定義されています。また、データベースと対話するためのCLIも含まれています。

## ビルドと実行

このプロジェクトでは `Makefile` を使用して、一般的な開発およびデプロイタスクを効率化しています。

### ローカル開発

1.  **サービスの起動:** SpannerエミュレータとRedisを起動します。
    ```bash
    docker compose up -d
    ```

2.  **ローカルSpannerのセットアップ:** ローカルのSpannerインスタンスとデータベースを作成し、スキーマを適用します。
    ```bash
    # エミュレータホストの設定
    export SPANNER_EMULATOR_HOST=localhost:9010
    export GOOGLE_CLOUD_PROJECT=your-project-id

    # インスタンスとデータベースの作成
    gcloud spanner instances create test-instance --config=emulator-config --description="test Instance" --nodes=1
    gcloud spanner databases create --instance test-instance game

    # スキーマの適用
    for schema in ./schemas/*.sql; do
        spanner-cli -p $GOOGLE_CLOUD_PROJECT -i test-instance -d game < $schema
    done
    ```

3.  **アプリケーションの実行:**
    ```bash
    export SPANNER_STRING=projects/$GOOGLE_CLOUD_PROJECT/instances/test-instance/databases/game
    PORT=8080 go run ./cmd/api/
    ```

### テスト

*   **ユニットテストの実行:**
    ```bash
    go test -v ./...
    ```
*   **手動APIテスト (サーバー実行中):**
    ```bash
    # Ping
    curl http://localhost:8080/ping

    # ユーザー作成
    curl http://localhost:8080/api/user/foo -X POST

    # ユーザーにアイテムを追加
    USER_ID=<your_user_id>
    ITEM_ID=d169f397-ba3f-413b-bc3c-a465576ef06e
    curl http://localhost:8080/api/user_id/$USER_ID/$ITEM_ID -X PUT
    ```

### Google Cloudへのデプロイ

`Makefile` がデプロイを自動化するためのターゲットを提供しています。

1.  **環境変数の設定:**
    ```bash
    export TF_VAR_project=<your-gcp-project>
    export TF_VAR_domain=<your-fqdn>
    # ... その他README.mdで定義されている変数
    ```

2.  **フルデプロイの実行:** このコマンドはTerraformでインフラをプロビジョニングし、データベーススキーマをセットアップし、Cloud Runサービスをデプロイします。
    ```bash
    make all
    ```

3.  **アプリケーションのみデプロイ:** 既存のインフラにアプリケーションコードのみをデプロイする場合:
    ```bash
    make app
    ```

## 開発規約

*   **プロジェクト構造:** コードは標準的なGoの慣習に従って構成されており、アプリケーションのエントリーポイントは `cmd` ディレクトリにあります。
*   **インフラストラクチャ:** すべてのGoogle Cloudインフラは `terraform/` ディレクトリにあるTerraformコードを介して管理されます。
*   **データベースマイグレーション:** データベーススキーマ（DDLとDML）は `.sql` ファイルとして `schemas/` ディレクトリに保存され、`spanner-cli` を使って適用されます。
*   **タスク自動化:** `Makefile` がテストの実行、アプリケーションのビルド、デプロイ管理の中心的な役割を担い、すべての開発者に一貫したインターフェースを提供します。