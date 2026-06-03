# WhatsApp Recorder

Автоматическая запись звонков WhatsApp Desktop в Windows.

## Установка

1. Скачайте `ffmpeg.exe` с https://www.gyan.dev/ffmpeg/builds/ (раздел **ffmpeg-release-essentials.zip**), извлеките `ffmpeg.exe`.
2. Положите `whatsapp-recorder.exe` и `ffmpeg.exe` в одну папку.
3. Запустите `whatsapp-recorder.exe`.

Приложение появится в системном трее. Запись начинается автоматически при входящем/исходящем звонке WhatsApp.

## Где хранятся записи

По умолчанию: `%USERPROFILE%\Documents\WhatsApp Calls`

Имя файла: `YYYY-MM-DD_HH-MM-SS.mp3`

## Изменить папку для записей

Правый клик по иконке в трее → **Settings...** — откроется `config.json` в текстовом редакторе.

Измените поле `output_folder`:
```json
{
  "output_folder": "D:\\Recordings\\WhatsApp",
  ...
}
```

Перезапустите приложение, чтобы изменения вступили в силу.

## Как собрать из исходников

Требования: Go 1.21+, доступ в интернет (для загрузки зависимостей).

```cmd
cd whatsapp-recorder
go mod tidy
go build -ldflags="-H windowsgui" -o whatsapp-recorder.exe .
```

## Удаление

1. Правый клик по иконке → **Start with Windows** (снять галку, если стоит).
2. Правый клик → **Exit**.
3. Удалите папку с `whatsapp-recorder.exe` и `ffmpeg.exe`.

Ключ реестра (если остался): `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` → `WhatsAppRecorder`.
