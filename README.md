# Alicloud APIs

The `alicloudapislim` Go package provides tools to interact with the logistics tracking APIs
provided by Alicloud. It allows you to fetch information about logistics providers and track
logistics status by number.

## Features

- List all available logistics providers.
- Get logistics providers by a tracking number.
- Get detailed logistics status by provider code and tracking number.

## Usage

First, initialize a new `WuliuClient` with your `AppCode`.

```go
client := alicloudapislim.NewWuliuClient("your_app_code_here")
```

### Get Providers

Fetch a list of all logistics providers:

```go
providers, err := client.GetProviders(context.Background())
if err != nil {
    // Handle error
}
```

Or, to get providers for a specific tracking number:

```go
providers, err := client.GetProvidersForNumber(context.Background(), "tracking_number_here")
if err != nil {
    // Handle error
}
```

### Get Logistics Status

To fetch logistics status:

```go
status, err := client.GetStatusForNumber(context.Background(), "provider_code_here", "tracking_number_here")
if err != nil {
    // Handle error
}
```

The `status` will contain detailed information such as updates, timestamps, and contact information.
