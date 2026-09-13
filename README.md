# TCB 3D DOC — Go/Ebitengine

Adaptation en Go de la démo 3D DOC de TCB, avec musique YM et cible Android.

## Lancer sur ordinateur

```sh
go run ./cmd/threeddoc
```

Les flèches haut et bas règlent le volume.

## Lancer sur le Pixel

Avec le Pixel branché, déverrouillé et autorisé pour le débogage USB :

```sh
./scripts/run-android.sh
```

Le script génère l’AAR Go/Ebitengine pour `arm64-v8a`, compile l’APK,
l’installe et lance `com.olivierh.threeddoc/.MainActivity`.

Prérequis validés : Go 1.27, OpenJDK 17, SDK Android 36, NDK
28.2.13676358 et un appareil Android autorisé dans `adb devices -l`.

Le guide détaillé se trouve dans
[`GUIDE_ANDROID_EBITENGINE_PIXEL.md`](GUIDE_ANDROID_EBITENGINE_PIXEL.md).

## Vérifier le code

```sh
go test ./...
go vet ./...
```
