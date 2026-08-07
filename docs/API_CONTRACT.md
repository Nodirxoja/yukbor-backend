# YUK BOR — API Contract (iOS ⇄ Backend)

Версия: 1.1 · Дата: 2026-08-07 · Статус: draft для синхронизации с backend-разработчиком

Это описание контрактов данных и эндпоинтов, которые ожидает iOS-приложение.
Сейчас все данные генерируются mock-сервисами внутри приложения
(`MockAuthService`, `MockOrderService`, `MockWalletService`, `MockNotificationService`,
`MockReviewService`) — этот документ формализует то же самое как реальный REST API,
чтобы backend можно было писать параллельно и потом просто подменить mock на HTTP-клиент.

Общие соглашения:
- Формат: JSON, `Content-Type: application/json`.
- Все идентификаторы — `UUID` (строка, RFC 4122).
- Все даты — ISO 8601 с таймзоной, напр. `2026-08-07T13:40:00Z`.
- Деньги — `Decimal`/строка с точностью до сум (UZS), без плавающей точки на бэке.
- Авторизация: `Authorization: Bearer <access_token>` во всех запросах, кроме `/auth/otp/*`.
- Ошибки: единый формат ниже.

```json
{
  "error": {
    "code": "OTP_INVALID",
    "message": "Неверный код подтверждения"
  }
}
```

---

## 1. Аутентификация (`/auth`)

Роли (`UserRole`): `client`, `driver`, `equipmentProvider`, `laborProvider`, `fleetAdmin`, `admin`.
Важно: `driver`/`equipmentProvider`/`laborProvider` — это три РАЗНЫХ типа исполнителей
(перевозка / спецтехника / рабочая сила), регистрируются одним и тем же флоу, отличается только `role`.

### POST /auth/otp/request
Запросить SMS-код.

Request:
```json
{ "phoneNumber": "+998901234567" }
```
Response `200`:
```json
{ "verificationId": "b2f1e2b0-....", "expiresInSeconds": 120 }
```

### POST /auth/otp/verify
Response `200`:
```json
{ "verificationId": "b2f1e2b0-....", "code": "1234" }
```
```json
{ "verified": true }
```
Ошибки: `OTP_INVALID`, `OTP_EXPIRED`.

### POST /auth/myid/verify
Верификация личности через **MyID** (единая система биометрической идентификации Узбекистана,
myid.uz) — обязательный шаг KYC после подтверждения телефона и перед созданием аккаунта
(TRD п.2.2). Клиент отправляет паспортные данные + селфи, сервер проксирует запрос в MyID API
и возвращает результат сверки лица с фото в документе/ГЦП.

Request (`multipart/form-data`, т.к. включает изображение селфи):
```
verificationId: "b2f1e2b0-...."   // тот же, что и в otp/verify — привязка к номеру телефона
passportSeries: "AB"
passportNumber: "1234567"
pinfl: "12345678901234"           // ПИНФЛ, 14 цифр
birthDate: "2001-03-15"
selfie: <binary jpeg/png>
```
Response `200`:
```json
{
  "myIdVerificationToken": "myid_tok_9f8a....",
  "isMatched": true,
  "confidence": 0.98,
  "verifiedFullName": "Karimov Aziz Baxtiyorovich"
}
```
- `myIdVerificationToken` — короткоживущий токен (TTL ~10 мин), подтверждающий успешную
  верификацию; передаётся в `POST /auth/register` вместо повторной отправки паспорта/селфи.
- `verifiedFullName` — ФИО из MyID; рекомендуется сверять/автозаполнять `fullName` при регистрации,
  чтобы избежать расхождений между тем, что ввёл пользователь, и официальными данными.
- Если `isMatched: false` — регистрация должна быть заблокирована на клиенте (экран пересъёмки).

Ошибки: `PASSPORT_NOT_FOUND` (нет такого паспорта/ПИНФЛ в ГЦП), `FACE_MISMATCH` (лицо не совпадает),
`MYID_SERVICE_UNAVAILABLE` (сервис MyID недоступен — клиент должен показать retry).

> **Интеграционная деталь для backend**: MyID предоставляет отдельный B2B/B2G API
> (OAuth2 client-credentials + REST) для верификации по ПИНФЛ и биометрии — необходимо оформить
> доступ через myid.uz/партнёрский кабинет. Наш `/auth/myid/verify` — это обёртка над их API,
> клиентское приложение никогда не обращается к MyID напрямую.

### POST /auth/register
Создание нового пользователя после успешной верификации телефона **и** личности через MyID.

Request:
```json
{
  "fullName": "Aziz Karimov",
  "phoneNumber": "+998901234567",
  "role": "driver",
  "myIdVerificationToken": "myid_tok_9f8a...."
}
```
Response `201` → объект `User` (см. §5) + токены:
```json
{
  "user": { "...": "см. User", "verificationStatus": "approved" },
  "accessToken": "eyJ...",
  "refreshToken": "eyJ..."
}
```
Ошибки: `PHONE_ALREADY_REGISTERED`, `MYID_TOKEN_EXPIRED_OR_INVALID` (нужно повторно пройти MyID-верификацию).

> После успешного MyID-прохождения `verificationStatus` пользователя сразу `approved`
> (в отличие от прежней ручной модерации документов) — это ключевое ускорение онбординга.

### POST /auth/login
Вход по уже зарегистрированному телефону (после OTP).
Request: `{ "phoneNumber": "+998901234567" }`
Response `200`: то же, что `/auth/register` (user + accessToken + refreshToken).
Ошибки: `USER_NOT_FOUND`.

### POST /auth/refresh
Request: `{ "refreshToken": "..." }` → Response: `{ "accessToken": "...", "refreshToken": "..." }`

### POST /auth/logout
Инвалидирует refresh-токен. Response `204`.

---

## 2. Пользователь (`User`)

```json
{
  "id": "3fa85f64-....",
  "role": "driver",
  "fullName": "Aziz Karimov",
  "phoneNumber": "+998901234567",
  "email": null,
  "isVerified": true,
  "verificationStatus": "approved",
  "rating": 4.8,
  "ratingsCount": 23
}
```
`verificationStatus`: `pending | approved | rejected` (проверка документов исполнителя — паспорт/права/техпаспорт техники, TRD).

### GET /users/me
Текущий профиль (по токену). Response `200` → `User`.

### PATCH /users/me
Частичное обновление (fullName/email). Response `200` → `User`.

---

## 3. Заказы (`/orders`)

### 3.1 Модель `Order`

Ключевая модель — три услуги в одной платформе. Комбо-заказ (`transportWithOptions`)
может содержать **два независимых исполнителя одновременно** (водитель + спецтехника
и/или водитель + рабочая сила) — у каждого своё плечо (`leg`) со своим статусом
и своим исполнителем. Это критично: escrow-выплата и статус-трекинг ведутся отдельно
по каждому плечу, а не по заказу в целом.

```json
{
  "id": "d290f1ee-....",
  "clientId": "3fa85f64-....",
  "clientName": "Dilnoza Yusupova",
  "type": "transportWithOptions",
  "status": "inTransit",
  "cargo": {
    "cargoType": "Стройматериалы",
    "weightTons": 12.5,
    "requiresRefrigeration": false,
    "requiredVehicleType": "flatbed",
    "specialInstructions": "Хрупкий груз"
  },
  "equipmentRequest": {
    "equipmentType": "crane",
    "durationHours": 4,
    "notes": "Кран для разгрузки"
  },
  "laborRequest": {
    "workersCount": 3,
    "durationHours": 2,
    "taskDescription": "Погрузка/разгрузка"
  },
  "pickupAddress": "Ташкент, Юнусабадский р-н, ул. Амира Темура 15",
  "pickupLocation": { "latitude": 41.311081, "longitude": 69.240562 },
  "dropoffAddress": "Самарканд, ул. Регистан 5",
  "dropoffLocation": { "latitude": 39.654896, "longitude": 66.959843 },
  "scheduledDate": "2026-08-10T09:00:00Z",
  "priceEstimate": "1250000",
  "currency": "UZS",

  "assignedDriverId": "7c9e6679-....",
  "assignedDriverName": "Aziz Karimov",

  "assignedEquipmentProviderId": "9b2e6679-....",
  "assignedEquipmentProviderName": "Botir Rashidov",
  "equipmentStatus": "loadingInProgress",

  "assignedLaborProviderId": null,
  "assignedLaborProviderName": null,
  "laborStatus": null,

  "createdAt": "2026-08-07T10:00:00Z",
  "updatedAt": "2026-08-07T13:20:00Z"
}
```

**`type` (`OrderType`)**:
| Значение | Значение на русском |
|---|---|
| `transportOnly` | Только перевозка (водитель) |
| `transportWithOptions` | Комбо: перевозка + спецтехника и/или рабочая сила |
| `equipmentOnly` | Только спецтехника |
| `laborOnly` | Только рабочая сила |

**`status` / `equipmentStatus` / `laborStatus` (`OrderStatus`)** — общий enum для всех трёх плеч:
`draft, published, matched, accepted, inProgress, loadingInProgress, inTransit, delivered, completed, cancelled, disputed`

> UI показывает разные подписи в зависимости от `orderType` для `loadingInProgress/inTransit/delivered`
> (напр. для `equipmentOnly`: "Техника на месте, работы начаты" / "Работы выполняются" / "Работы завершены").
> Названия — только для отображения, backend хранит и возвращает "сырой" enum-статус.

**Плечи (`leg`)**: `transport | equipment | labor`. У `transportOnly/equipmentOnly/laborOnly` — одно плечо.
У `transportWithOptions` — от 1 до 3 плеч в зависимости от того, заполнены ли `equipmentRequest`/`laborRequest`.

**`EquipmentType`**: `excavator, crane, forklift, loader`
**`VehicleType`**: `tractorTrailer, flatbed, refrigerated, tanker, dumpTruck, boxTruck`
**`PaymentMethod`** (см. §4): `payme, click, uzcard`

### 3.2 Эндпоинты

#### POST /orders
Создать заказ (клиент). Body — `Order` без `id/status/createdAt/updatedAt` (сервер их проставляет,
`status = draft` → сразу переводится в `published`).
Response `201` → `Order`.

#### GET /orders?clientId={uuid}
Заказы конкретного клиента (для экрана "Мои заказы"). Response `200` → `Order[]`.

#### GET /orders/available?leg={transport|equipment|labor}
Открытая лента заказов для исполнителя данного плеча: `status(for: leg) == published`
и `assignedExecutorId(for: leg) == null`. Response `200` → `Order[]`.

#### GET /orders/{id}
Response `200` → `Order`. `404 ORDER_NOT_FOUND`.

#### POST /orders/{id}/accept
Исполнитель принимает СВОЁ плечо заказа.
```json
{ "leg": "equipment", "executorId": "9b2e6679-....", "executorName": "Botir Rashidov" }
```
Сервер: проставляет `assigned{Leg}Id/Name`, переводит статус этого плеча в `accepted`.
Response `200` → обновлённый `Order`.
Ошибки: `LEG_ALREADY_TAKEN`, `ORDER_NOT_PUBLISHED`.

#### PATCH /orders/{id}/status
Исполнитель обновляет статус СВОЕГО плеча (напр. начал погрузку → в пути → доставлено).
```json
{ "leg": "transport", "status": "inTransit" }
```
Response `200` → обновлённый `Order`.

#### POST /orders/{id}/cancel
Отмена заказа клиентом (только пока не начато исполнение). Response `200` → `Order`.

#### POST /orders/{id}/confirm-completion
Клиент подтверждает выполнение — вызывается, когда **все плечи** заказа в статусе
`delivered`/`completed` (`isReadyForClientConfirmation`). Сервер помечает все плечи `completed`
и запускает выплату escrow **каждому** исполнителю по отдельности (см. §4).
Response `200` → `Order`.

#### GET /orders/backhaul?dropoffLat={}&dropoffLng={}&excludeOrderId={}
Ключевая фича-дифференциатор: обратный рейс — поиск открытых заказов рядом с точкой
выгрузки водителя (радиус ~15 км), чтобы не возвращаться порожняком.
Response `200` → `Order[]`, отсортированы по расстоянию.

---

## 4. Кошелёк / Escrow (`/wallet`)

Модель `Transaction` — деньги замораживаются (`held`) при принятии заказа клиентом (оплата),
и переводятся исполнителю (`released`) только после `confirm-completion`.
**Важно для комбо-заказов**: на один `orderId` может быть несколько транзакций —
по одной на каждого исполнителя (`payeeId`), с независимым освобождением.

```json
{
  "id": "aa1e6679-....",
  "orderId": "d290f1ee-....",
  "orderTitle": "Стройматериалы",
  "payerId": "3fa85f64-....",
  "payeeId": "7c9e6679-....",
  "amount": "1250000",
  "platformCommission": "125000",
  "paymentMethod": "payme",
  "status": "held",
  "createdAt": "2026-08-07T10:05:00Z",
  "releasedAt": null
}
```
`status`: `held | released | refunded`. `payoutAmount = amount - platformCommission`.

### POST /wallet/transactions
Создать escrow-транзакцию при оформлении заказа/принятии исполнителем.
```json
{
  "orderId": "...", "orderTitle": "...", "payerId": "...", "payeeId": "...",
  "amount": "1250000", "paymentMethod": "payme"
}
```
Response `201` → `Transaction`.

### POST /wallet/transactions/release
```json
{ "orderId": "...", "payeeId": "..." }
```
Переводит именно эту транзакцию (одного исполнителя) в `released`, `releasedAt = now()`.
Response `200` → `Transaction`.

### GET /wallet/transactions?payeeId={uuid}
История транзакций исполнителя (для экрана "Кошелёк": доступный баланс = сумма `released`,
"в escrow" = сумма `held`). Response `200` → `Transaction[]`.

---

## 5. Уведомления (`/notifications`)

```json
{
  "id": "cc1e6679-....",
  "userId": "3fa85f64-....",
  "type": "backhaulSuggestion",
  "title": "Обратный рейс рядом",
  "body": "Найден заказ в 3 км от точки выгрузки",
  "relatedOrderId": "d290f1ee-....",
  "isRead": false,
  "createdAt": "2026-08-07T13:00:00Z"
}
```
`type`: `newOrderMatch | orderStatusChanged | paymentReleased | backhaulSuggestion | reviewReceived`

- `GET /notifications?userId={uuid}` → `AppNotification[]`
- `PATCH /notifications/{id}/read` → `204`
- Реалтайм: рекомендуется WebSocket/push (FCM/APNs) канал `user.{id}.notifications`,
  события того же формата, что и элементы списка выше (см. §7 realtime).

---

## 6. Отзывы (`/reviews`)

```json
{
  "id": "dd1e6679-....",
  "orderId": "d290f1ee-....",
  "reviewerId": "3fa85f64-....",
  "revieweeId": "7c9e6679-....",
  "rating": 5,
  "comment": "Всё отлично, быстро и аккуратно",
  "createdAt": "2026-08-07T15:00:00Z"
}
```

- `POST /reviews` → создать отзыв после `completed`. Response `201` → `Review`.
- `GET /reviews/rating?userId={uuid}` → `{ "rating": 4.8, "count": 23 }` (агрегат для `User.rating/ratingsCount`).

---

## 7. Realtime / синхронизация состояния

Сейчас на клиенте всё построено на Combine `Publisher`-ах (`ordersPublisher`,
`transactionsPublisher`, `notificationsPublisher`), которые должны получать push-обновления,
а не только отвечать на запрос. Backend должен обеспечить один из вариантов:

1. **WebSocket** (предпочтительно для хакатона — проще всего): один канал на пользователя,
   события: `order.updated`, `order.created`, `transaction.updated`, `notification.created`.
   Payload события — тот же JSON-объект, что и в REST-ответах выше, плюс `"event"` тип.
2. Или **polling** каждые 5–10 сек по `GET /orders?...` / `GET /notifications?...` как fallback.

Пример WS-сообщения:
```json
{ "event": "order.updated", "data": { "...": "Order JSON" } }
```

---

## 8. Что не входит в MVP backend (пока мокается на клиенте)

- Расчёт цены (`PriceEstimator`) — сейчас чистая клиентская формула
  (вес × тариф/т + часы спецтехники × ставка + рабочие × часы × ставка, минимум 100 000 UZS).
  **Обсудить с backend**: скорее всего расчёт должен переехать на сервер (тарифы могут меняться,
  нужна единая логика), клиент будет слать только черновой запрос и получать `priceEstimate` в ответ
  на `POST /orders/estimate` (эндпоинт предлагается добавить).
- Реальный geocoding адресов (сейчас `PseudoGeocoder` — заглушка). Нужен реальный сервис
  геокодирования (Яндекс/Google/2GIS Maps API) на этапе создания заказа и на карте выбора адреса.
- Реальная оплата через Payme/Click/Uzcard — сейчас просто enum `PaymentMethod`, без интеграции.
- Реальная интеграция с MyID (`MockMyIDVerificationService` на клиенте просто эмулирует задержку
  ~2 сек и всегда возвращает успех) — backend должен подключить настоящий MyID B2B/B2G API
  и реализовать `POST /auth/myid/verify` как прокси к нему (см. §1).

---

## 9. Чеклист для backend-разработчика

- [ ] Auth: OTP request/verify, register, login, refresh, logout, роли `client/driver/equipmentProvider/laborProvider/fleetAdmin/admin`
- [ ] **MyID KYC**: `POST /auth/myid/verify` (проксирование паспорт+селфи в MyID API), `myIdVerificationToken` в `POST /auth/register`
- [ ] Users: `GET/PATCH /users/me`
- [ ] Orders CRUD + accept/status/cancel/confirm-completion **с учётом плеч (`leg`)**
- [ ] Backhaul-поиск `GET /orders/backhaul`
- [ ] Wallet: escrow create/release **на пару (orderId, payeeId)**, история транзакций
- [ ] Notifications: список, read, realtime-канал
- [ ] Reviews: создание + агрегированный рейтинг
- [ ] Realtime-канал (WebSocket предпочтительно) для orders/transactions/notifications
- [ ] Файлы Swift-моделей для сверки полей 1:1: `Core/Model/{Order,User,Transaction,NotificationModel,ReviewModel}.swift`,
      `Domain/Auth/MyIDVerificationService.swift` (контракт MyID-верификации)
