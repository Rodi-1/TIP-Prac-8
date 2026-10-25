# ПЗ №8 — MongoDB и REST-сервис заметок

Сортов Семён Александрович, ЭФМО-01-26. Дата: 25.10.2026.

Реализованы все пять CRUD-операций, уникальный title, тайм-ауты, тест Create/ByID и все четыре задания со звёздочкой: текстовый поиск, TTL, курсорная пагинация и агрегация статистики.

Требования: Go 1.24+, Docker Desktop, PowerShell или curl/Postman. Проверено с Go 1.27.2 на Windows 11 amd64 и MongoDB 7. Использован драйвер mongo-driver v1, соответствующий листингам задания.

## Запуск

```powershell
docker compose up -d
docker compose exec mongo mongosh --quiet -u root -p secret --authenticationDatabase admin --eval 'db.runCommand({ping:1})'
Copy-Item .env.example .env
go run ./cmd/api
```

По умолчанию Go использует значения из .env.example: Mongo на localhost:27017, база pz8, HTTP :8080. Сам файл .env автоматически не загружается; для других параметров задайте переменные в текущем окне:

```powershell
$env:MONGO_URI='mongodb://root:secret@localhost:27017/?authSource=admin'
$env:MONGO_DB='pz8'
$env:HTTP_ADDR=':8080'
```

root/secret — данные локального учебного контейнера. API других практик на порту 8080 запускаются отдельно.

## Пять операций

В другом окне PowerShell:

```powershell
$url='http://localhost:8080/api/v1/notes'
$note=Invoke-RestMethod $url -Method Post -ContentType 'application/json' -Body '{"title":"First note","content":"Hello Mongo!"}'
# 201, id — строка ObjectID, title, content, createdAt, updatedAt
Invoke-RestMethod ($url+'?limit=5&skip=0&q=first')
# 200, массив с созданной заметкой
Invoke-RestMethod ($url+'/'+$note.id)
# 200, заметка по id
Invoke-RestMethod ($url+'/'+$note.id) -Method Patch -ContentType 'application/json' -Body '{"content":"Updated content"}'
# 200, content обновлён; createdAt сохранён
Invoke-WebRequest ($url+'/'+$note.id) -Method Delete -UseBasicParsing
# 204, пустое тело
```

Повтор title возвращает 409, некорректный JSON или пустой заголовок — 400, неизвестный или некорректный ObjectID — 404. Пустая выборка возвращается как [], не null.

## Все задания со звёздочкой

1. Текстовый индекс title. Параметр q использует `$text: {$search: q}`. Поиск выполняется по словам заголовка, а не по произвольной подстроке.
2. TTL: необязательное поле expiresAt в формате RFC3339, индекс expiresAt:1 с expireAfterSeconds:0. Например:

```powershell
$body=@{title='Temporary note';content='TTL example';expiresAt=[DateTime]::UtcNow.AddSeconds(10).ToString('o')} | ConvertTo-Json
Invoke-RestMethod $url -Method Post -ContentType 'application/json' -Body $body
```

Удаление выполняет фоновый TTL-монитор MongoDB, поэтому оно происходит не строго в момент expiresAt. Без expiresAt заметка автоматически не удаляется.

3. Курсор: GET /api/v1/notes?limit=2&after=ObjectID. Используется фильтр `_id < after`, сортировка `_id:-1`. После первой страницы передайте id последней заметки. Обычные limit/skip остаются доступны; after нельзя сочетать с ненулевым skip. limit: 1–200, по умолчанию 20.
4. GET /api/v1/notes/stats — aggregation pipeline с количеством и средней длиной content в символах:

```json
{"count":3,"averageContentLength":8}
```

Поля времени перед сохранением приводятся к миллисекундам BSON, поэтому созданный объект и последующее чтение возвращают одинаковый createdAt.

## Тесты

```powershell
$env:MONGO_URI='mongodb://root:secret@localhost:27017/?authSource=admin'
go test -v ./...
go vet ./...
go build -o bin/app.exe ./cmd/api
```

TestCreateAndGet проверяет Create/ByID и уникальный индекс в отдельной базе pz8_test_<случайный ObjectID>, после теста удаляет только эту базу. Без MONGO_URI тест пропускается. С переменной он действительно обращается к MongoDB.

## Типовые проблемы

- Connection refused: запустить MongoDB и дождаться ответа ping.
- Ошибка аутентификации: проверить root/secret и authSource=admin.
- Duplicate key / 409: title уже занят; выбрать другой.
- ObjectID должен содержать 24 шестнадцатеричных символа; неверный ID даёт 404.
- TTL не удалил заметку мгновенно: дождаться фонового прохода TTL-монитора.
