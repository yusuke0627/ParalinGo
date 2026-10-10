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

AI接続を設定すると自動翻訳・サマリ・目標レベルに合わせた表現解説を生成できます。復習生成は未実装です。

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

## URLから読む

デスクトップ版の「URLから読む」で http/https の記事を開きます。外部ページはMyGOの別ウィンドウで表示します。読書ウィンドウへ戻り「表示中の記事を取り込む」を押すと、タイトル・現在のURL・表示中の段落を取得して編集画面へ進みます。英文は文ごとに改行され、訳なしでも保存できます。

戻る・進む・再読込はURLダイアログから利用できます。本文抽出は `article` / `main` の段落を優先する初期実装です。サイトによってメニューの混入や本文の取りこぼしがあるため、保存前に編集してください。iframe内の本文、段落タグを使わないサイト、別窓リンクは未対応です。Webプレビューでは利用できません。

## ChatGPTプランで生成

1. デスクトップ版の「ChatGPT接続」を開く。
2. 「Continue with ChatGPT」を押し、システムブラウザの公式画面でログインとプラン利用の許可を行う。
3. ChatGPT側の利用設定でParalinGoの追加クレジット利用を無効にする。プラン連携はクレジット利用も提供するため、サブスクのみの予算管理はChatGPT側で設定する。
4. アプリへ戻り「モデル一覧を取得」、利用可能なモデルを選ぶ。
5. 記事で「対訳・解説を生成」を押す。

APIキーの入力と従量課金APIへのフォールバックはありません。上限・認証エラー時は停止します。生成結果の保存形式は以前と同じです。

OAuthはPKCE、state、nonce、署名・issuer・audience・期限を検証。認証トークンはGoのメモリ内で保持し、再起動時は再ログインします。ホストIDとクライアント登録情報だけをOS設定フォルダの `ParalinGo/chatgpt-registration.json` に保存します。ログアウトは公式のセッション失効も試み、確認できない場合はChatGPT設定から解除する案内を表示します。

対象プランと実アカウントでの提供状況は公式ログイン後に確認が必要です。署名付きログイン・実生成はユーザーの許可後に実機確認する必要があります。

- https://developers.openai.com/siwc/token-sharing-open-source/sign-in
- https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference
- https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations
