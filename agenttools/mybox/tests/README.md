# `tests/`

このディレクトリには、CLIのエンドツーエンドテストと、インストール先の安全ガードを検証するテストを置きます。Web UIのユニットテストとPlaywrightテストは [`../web/README.md`](../web/README.md) を参照してください。

## Index

| パス | 役割 |
| --- | --- |
| [`cli_e2e.sh`](./cli_e2e.sh) | プロジェクト登録、タスク、ファイル操作などCLI一連の動作を検証。 |
| [`install_makefile_test.sh`](./install_makefile_test.sh) | `make install`のGOBINとworker配置先ガードを一時ディレクトリで検証。 |

インストール先ガードのテストは、次のコマンドで実行できます。Web UIのビルドやChromiumのダウンロードは行いません。

```bash
make test-install
```

