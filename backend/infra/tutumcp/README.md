# tutumcp

Go-клиент [Tutu MCP](https://mcp.tutu.ru/mcp): поиск авиабилетов, поездов, автобусов, электричек и отелей, детали предложений, схема мест в поезде и ссылки на оформление.

Без внешних зависимостей. Транспорт — JSON-RPC 2.0 поверх MCP Streamable HTTP, авторизация не требуется.

## Установка

```bash
go get github.com/tutu-hack/openworld/infra/tutumcp
```

## Быстрый старт

Клиент создаётся один раз при старте сервиса и передаётся зависимостям — он потокобезопасен.

```go
client, err := tutumcp.New(
    tutumcp.WithClientInfo("tutu-plan", "1.0.0"),
    tutumcp.WithTimeout(90*time.Second),
)
if err != nil {
    return err
}
defer client.Close(context.Background())

if _, err := client.Initialize(ctx); err != nil {
    return err
}
```

`Initialize` можно не вызывать: рукопожатие произойдёт при первом обращении к инструменту, а при потере сессии повторится само. Явный вызов удобен, чтобы проверить доступность сервера на старте.

## Поиск

```go
res, err := client.SearchRail(ctx, tutumcp.RailParams{
    SearchParams: tutumcp.SearchParams{
        Origin:        "Москва",
        Destination:   "Санкт-Петербург",
        DepartureDate: "2026-09-20",
        View:          tutumcp.ViewCompact,
        PageSize:      5,
    },
    Passengers: 1,
})
if err != nil {
    return err
}

for _, offer := range res.Items() {
    fmt.Println(offer.Title(), offer.Price.String(), offer.DurationMin)
}
```

`Items()` возвращает предложения независимо от того, назвал ли их домен `offers`, `variants` или `hotels`. Есть помощники `Cheapest()`, `Fastest()`, `HasMore()`, а полный ответ сервера всегда лежит в `res.Raw`.

Постранично:

```go
err := tutumcp.Paginate(ctx, 3, func(ctx context.Context, page int) (*tutumcp.SearchResult, error) {
    params.Page = page
    return client.SearchRail(ctx, params)
}, func(page *tutumcp.SearchResult) error {
    offers = append(offers, page.Items()...)
    return nil
})
```

## Детали и оформление

```go
details, err := client.OfferDetailsFor(ctx, tutumcp.ProductRail, offer)
seatmap, err := client.GetRailSeatmap(ctx, tutumcp.SeatmapParams{
    DetailsRef: offer.DetailsRef,
    Task:       tutumcp.SeatmapTogether,
})
link, err := client.CheckoutLinkFor(ctx, tutumcp.ProductRail, offer)
```

`link.Kind` обязательно учитывать в интерфейсе: `deeplink` ведёт на оформление выбранного предложения, `search_redirect` — только на страницу поиска.

Поля `DetailsRef` и `CheckoutRef` непрозрачны — они хранятся как `json.RawMessage` и передаются обратно без изменений. Собирать URL вручную не нужно.

Места и тариф добавляются к оформлению только после подтверждения пользователем:

```go
link, err := client.CreateCheckoutLink(ctx, tutumcp.CheckoutParams{
    ProductType: tutumcp.ProductRail,
    CheckoutRef: offer.CheckoutRef,
    CarNumber:   "7",
    SeatNumbers: []string{"12", "14"},
})
```

## Справочники

```go
text, err := client.Instructions(ctx, tutumcp.DomainRail)
status, err := client.FetchResource(ctx, tutumcp.ResourceStatus)
tools, err := client.ListTools(ctx)
raw, err := client.CallTool(ctx, "search_avia", args)
```

`ListTools` отдаёт актуальные JSON Schema инструментов — схемы меняются на стороне сервера, фиксировать их в коде не стоит. `CallTool` пригодится для инструментов, ещё не покрытых типизированными методами.

## Ошибки

```go
if verr, ok := tutumcp.AsValidationError(err); ok {
    // параметры не прошли проверку до отправки запроса
}
if terr, ok := tutumcp.AsToolError(err); ok {
    // сервер вернул isError: город не распознан, источник недоступен
    log.Println(terr.Message())
}
if herr, ok := tutumcp.AsHTTPError(err); ok {
    log.Println(herr.StatusCode)
}
```

Сетевые сбои, `429` и `5xx` повторяются автоматически: три попытки с экспоненциальной паузой, джиттером и учётом `Retry-After`. Настраивается через `WithRetry` и `WithoutRetry`.

## Структура

| Пакет | Ответственность |
|---|---|
| `tutumcp` | предметная область Tutu: поиск, детали, места, checkout |
| `mcp` | протокол MCP: рукопожатие, инструменты, ресурсы |
| `transport` | Streamable HTTP: сессия, JSON и SSE, повторы |

Сервису обычно достаточно корневого пакета. Нижние слои пригодятся, если понадобится обратиться к другому MCP-серверу или подменить транспорт.

## Ограничения

Клиент только читает данные и создаёт ссылки: оплаты и бронирования на его стороне нет. В запросы нельзя передавать ФИО, документы, платёжные и контактные данные. Цены и наличие мест динамичны, поэтому долго кэшировать выдачу не следует, а `checkout_url` не является подтверждением покупки.

## Опции

| Опция | Назначение |
|---|---|
| `WithEndpoint` | другой адрес сервера |
| `WithHTTPClient` | свой `http.Client` с пулом, прокси, метриками |
| `WithTimeout` | таймаут запроса, по умолчанию 90 секунд |
| `WithClientInfo` | имя и версия приложения для сервера |
| `WithHeader` | дополнительные заголовки, например trace-id |
| `WithRetry`, `WithoutRetry` | политика повторов |
| `WithLogger` | `*slog.Logger` для диагностики на уровне Debug |
| `WithProtocolVersion` | фиксация версии протокола MCP |
| `WithMaxResponseBytes` | лимит размера ответа |
