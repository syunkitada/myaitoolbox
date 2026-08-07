# nginx: playground

## セットアップ方法

```
# Create playground
$ make

# Clean playground, when you no longer need it
$ make clean
```

## アクセス方法

### サーバのローカルからアクセスする場合

```
$ curl https://myapp.svc.local.test:10443/
```

### Windows リモート端末 からアクセスする場合

C:\Windows\System32\drivers\etc\hosts に以下の内容を追記してください。(IPは適宜変更してください)

```
/etc/hosts
192.168.10.121 myapp.svc.local.test
```

ブラウザから以下にアクセスできます。

https://myapp.svc.local.test:10443/

### 証明書エラーが出る場合

以下のURLから証明書をダウンロードして、ルート証明書を信頼させてください。

https://myapp.svc.local.test/download/ca.pem

Windowsの場合は、以下の手順で証明書を信頼させてください。

1. certmgr.msc
2. 「信頼されたルート証明機関」
3. 「証明書」
4. インポート
