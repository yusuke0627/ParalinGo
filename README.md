# ParalinGo

MyGOで作る、英語リーディングのためのデスクトップアプリ。

## 起動

Go 1.27.1以上、Bunが必要です（Goの自動toolchain取得にも対応）。

```sh
cd app
bun install
bun run dev
```

## 最初の版

- 英文一文と日本語訳を交互に表示。訳は常時表示・クリック表示・非表示を選択。
- 英文と日本語訳を一文ずつ改行して貼り付け、ライブラリに保存。
- 英文を選択し、表現とメモ、原文を保存。
- 各文のチェックで読書位置を保存し、「続きから読む」で復帰。
- Markdown書き出し、JSONバックアップと読み込み。
- MyGO版ではOSの設定フォルダの `ParalinGo/library.json` に保存。macOSでは通常 `~/Library/Application Support/ParalinGo/library.json`。保存は一時ファイルから置換。
- WebプレビューではlocalStorageに保存。デスクトップ版とは別の保存先。

自動翻訳・記事のURL取り込み・サマリ・復習生成・目標レベルに応じた解説は未実装。学習目標は現時点では記録用です。

## 検証

```sh
cd app
bun run typecheck
bun run build:web
go test ./...
```

## 開発順

1. MyGO上で対訳を読み、保存し、再起動して読み返す。
2. 外部ページの閲覧と本文抽出を実機検証し、読書モードへ取り込む。
3. 翻訳サービス接続、チャンク説明・レベル調整。
4. サマリ・原文に基づく復習問題の生成。

MyGO評価記録は `docs/mygo-evaluation.md` に残します。
