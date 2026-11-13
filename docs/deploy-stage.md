# Deploy the sample to North Central US (Stage)

This guide reproduces the validated Azure Functions Go worker-image flow in a
subscription enabled for the internal Flex Consumption preview.

> [!IMPORTANT]
> This is a preview deployment procedure, not a generally available public
> Azure path. If `northcentralusstage` is absent from the Go runtime inventory,
> your subscription is not enabled for this procedure.

The commands use PowerShell 7.4 or later and Azure CLI 2.92 or later. They keep
credentials in memory and create isolated resources so cleanup is scoped to one
resource group.

## 1. Set names and verify access

Sign in and select the subscription:

```powershell
az login

$subscription = '<subscription-id>'
az account set --subscription $subscription
```

Confirm that Go `1.0` and FC1 are available:

```powershell
az functionapp list-flexconsumption-runtimes `
  --runtime go `
  --location northcentralusstage `
  --output json
```

Generate unique names:

```powershell
$stamp = [DateTime]::UtcNow.ToString('yyMMddHHmmss')
$random = [Guid]::NewGuid().ToString('N').Substring(0, 4)
$suffix = "g$stamp$random"

$resourceGroup = "rg-gopdf-$suffix"
$storage = ("stgopdf$suffix").Substring(0, [Math]::Min(24, ("stgopdf$suffix").Length))
$appInsights = "appi-gopdf-$suffix"
$registry = "acrgopdf$suffix"
$plan = "plan-gopdf-$suffix"
$app = "func-gopdf-$suffix"
$repository = "go-html-to-pdf-$suffix"
$scopeMap = "go-$suffix-read"
$pullToken = "go-$suffix"
$tag = "sample-$stamp"
```

Check globally unique names before creating resources:

```powershell
az storage account check-name --name $storage
az acr check-name --name $registry
az functionapp list --query "[?name=='$app'].name"
```

## 2. Create the supporting resources

Resource groups cannot use the internal Stage location as their metadata
location. Create the group, storage, monitoring, and registry in
`northcentralus`:

```powershell
az group create `
  --name $resourceGroup `
  --location northcentralus `
  --tags sample=go-html-to-pdf retention=review

az storage account create `
  --resource-group $resourceGroup `
  --name $storage `
  --location northcentralus `
  --sku Standard_LRS `
  --kind StorageV2 `
  --min-tls-version TLS1_2 `
  --https-only true `
  --allow-blob-public-access false `
  --public-network-access Enabled

az monitor app-insights component create `
  --resource-group $resourceGroup `
  --app $appInsights `
  --location northcentralus `
  --kind web `
  --application-type web `
  --retention-time 90

az acr create `
  --resource-group $resourceGroup `
  --name $registry `
  --location northcentralus `
  --sku Basic `
  --admin-enabled false `
  --public-network-enabled true
```

Do not enable anonymous registry pulls or the ACR admin account.

## 3. Build and push the image

Get the registry hostname and sign in with Microsoft Entra ID:

```powershell
$registryServer = az acr show `
  --name $registry `
  --query loginServer `
  --output tsv

az acr login --name $registry
```

Build and push Linux AMD64:

```powershell
$imageName = "${registryServer}/${repository}:${tag}"

docker build `
  --platform linux/amd64 `
  --target final `
  --tag $imageName `
  .

docker push $imageName
```

Resolve and validate the immutable registry digest:

```powershell
$digest = az acr repository show `
  --name $registry `
  --image "${repository}:${tag}" `
  --query digest `
  --output tsv

if ($digest -notmatch '^sha256:[0-9a-f]{64}$') {
    throw "ACR returned an invalid digest: $digest"
}

$pinnedImage = "${imageName}@${digest}"
$pinnedImage
```

Use the manifest or index digest returned by ACR, not a local Docker
configuration ID.

## 4. Create the read-only pull credential

Create a scope map that can read only this repository:

```powershell
az acr scope-map create `
  --registry $registry `
  --name $scopeMap `
  --repository $repository content/read metadata/read `
  --output none

az acr token create `
  --registry $registry `
  --name $pullToken `
  --scope-map $scopeMap `
  --no-passwords `
  --output none
```

The password is generated later and never written to disk.

## 5. Prepare ordinary Functions deployment storage

Create the deployment container and capture required connection values in
memory:

```powershell
$storageConnection = az storage account show-connection-string `
  --subscription $subscription `
  --resource-group $resourceGroup `
  --name $storage `
  --query connectionString `
  --output tsv

if (-not $storageConnection) {
    throw 'Storage connection string was empty.'
}

$env:AZURE_STORAGE_CONNECTION_STRING = $storageConnection
try {
    az storage container create `
      --name deploymentpackage `
      --public-access off `
      --output none
}
finally {
    $env:AZURE_STORAGE_CONNECTION_STRING = $null
}

$appInsightsConnection = az monitor app-insights component show `
  --subscription $subscription `
  --resource-group $resourceGroup `
  --app $appInsights `
  --query connectionString `
  --output tsv

if (-not $appInsightsConnection) {
    throw 'Application Insights connection string was empty.'
}
```

## 6. Create an ordinary Go Flex app

The released CLI does not create an app in the internal Stage location. Use the
validated Resource Manager shapes instead.

Get an ARM access token:

```powershell
$accessToken = az account get-access-token `
  --subscription $subscription `
  --resource https://management.azure.com/ `
  --query accessToken `
  --output tsv

$headers = @{
    Authorization = [string]::Concat('Bearer ', $accessToken)
}
```

Create the dedicated FC1 plan:

```powershell
$planUri = "https://management.azure.com/subscriptions/$subscription/resourceGroups/$resourceGroup/providers/Microsoft.Web/serverfarms/${plan}?api-version=2024-04-01"

$planBody = @{
    location = 'North Central US (Stage)'
    kind = 'functionapp'
    tags = @{
        sample = 'go-html-to-pdf'
        retention = 'review'
    }
    sku = @{
        name = 'FC1'
        tier = 'FlexConsumption'
        size = 'FC1'
        family = 'FC'
        capacity = 0
    }
    properties = @{
        reserved = $true
        zoneRedundant = $false
    }
} | ConvertTo-Json -Depth 20

$planResult = Invoke-RestMethod `
  -Method Put `
  -Uri $planUri `
  -Headers $headers `
  -ContentType 'application/json' `
  -Body ([Text.Encoding]::UTF8.GetBytes($planBody))
```

Create the ordinary Go `1.0` app:

```powershell
$siteBase = "https://management.azure.com/subscriptions/$subscription/resourceGroups/$resourceGroup/providers/Microsoft.Web/sites/$app"
$siteUri = "${siteBase}?api-version=2026-08-01"

$siteBody = @{
    location = 'North Central US (Stage)'
    kind = 'functionapp,linux'
    tags = @{
        sample = 'go-html-to-pdf'
        retention = 'review'
    }
    properties = @{
        serverFarmId = "/subscriptions/$subscription/resourceGroups/$resourceGroup/providers/Microsoft.Web/serverfarms/$plan"
        httpsOnly = $true
        publicNetworkAccess = 'Enabled'
        clientAffinityEnabled = $false
        siteConfig = @{
            appSettings = @(
                @{
                    name = 'AzureWebJobsStorage'
                    value = $storageConnection
                }
                @{
                    name = 'DEPLOYMENT_STORAGE_CONNECTION_STRING'
                    value = $storageConnection
                }
                @{
                    name = 'APPLICATIONINSIGHTS_CONNECTION_STRING'
                    value = $appInsightsConnection
                }
                @{
                    name = 'FUNCTIONS_EXTENSION_VERSION'
                    value = '~4'
                }
            )
            ftpsState = 'Disabled'
            minTlsVersion = '1.2'
            scmMinTlsVersion = '1.2'
            http20Enabled = $true
        }
        functionAppConfig = @{
            runtime = @{
                name = 'go'
                version = '1.0'
            }
            scaleAndConcurrency = @{
                maximumInstanceCount = 2
                instanceMemoryMB = 2048
            }
            deployment = @{
                storage = @{
                    type = 'blobContainer'
                    value = "https://$storage.blob.core.windows.net/deploymentpackage"
                    authentication = @{
                        type = 'StorageAccountConnectionString'
                        storageAccountConnectionStringName = 'DEPLOYMENT_STORAGE_CONNECTION_STRING'
                    }
                }
            }
            siteUpdateStrategy = @{
                type = 'Recreate'
            }
        }
    }
} | ConvertTo-Json -Depth 30

$siteResult = Invoke-RestMethod `
  -Method Put `
  -Uri $siteUri `
  -Headers $headers `
  -ContentType 'application/json' `
  -Body ([Text.Encoding]::UTF8.GetBytes($siteBody))
```

Read the site back and verify the ordinary runtime before conversion:

```powershell
$site = Invoke-RestMethod -Method Get -Uri $siteUri -Headers $headers

if ($site.properties.functionAppConfig.runtime.name -ne 'go' -or
    $site.properties.functionAppConfig.runtime.version -ne '1.0') {
    throw 'Expected an ordinary Go 1.0 Flex app.'
}
```

## 7. Convert the app to the worker image

List the current app settings, refuse to overwrite existing registry setting
names, and generate a seven-day pull credential:

```powershell
$settingsBase = "${siteBase}/config/appsettings"
$settings = Invoke-RestMethod `
  -Method Post `
  -Uri "${settingsBase}/list?api-version=2026-08-01" `
  -Headers $headers

foreach ($name in @(
        'FLEX_CONTAINER_REGISTRY_USERNAME',
        'FLEX_CONTAINER_REGISTRY_PASSWORD'
    )) {
    if ($settings.properties.PSObject.Properties.Name -contains $name) {
        throw "Setting $name already exists; refusing to overwrite it."
    }
}

$credentialsJson = az acr token credential generate `
  --registry $registry `
  --name $pullToken `
  --password1 `
  --expiration-in-days 7 `
  --output json

$credentials = $credentialsJson | ConvertFrom-Json
$password = $credentials.passwords |
  Where-Object name -eq 'password1' |
  Select-Object -First 1

if (-not $credentials.username -or
    -not $password.value -or
    -not $password.expiry) {
    throw 'The generated pull credential is incomplete.'
}
```

Update the settings and convert the preserved `functionAppConfig`:

```powershell
try {
    $settings.properties | Add-Member `
      -NotePropertyName FLEX_CONTAINER_REGISTRY_USERNAME `
      -NotePropertyValue $credentials.username

    $settings.properties | Add-Member `
      -NotePropertyName FLEX_CONTAINER_REGISTRY_PASSWORD `
      -NotePropertyValue $password.value

    $settingsBody = @{
        properties = $settings.properties
    } | ConvertTo-Json -Depth 30

    $null = Invoke-RestMethod `
      -Method Put `
      -Uri "${settingsBase}?api-version=2026-08-01" `
      -Headers $headers `
      -ContentType 'application/json' `
      -Body ([Text.Encoding]::UTF8.GetBytes($settingsBody))

    $config = $site.properties.functionAppConfig
    $config.runtime = $null
    $config.deployment.storage = @{
        type = 'Registry'
        value = $pinnedImage
        authentication = @{
            type = 'Basic'
            usernameSettingName = 'FLEX_CONTAINER_REGISTRY_USERNAME'
            passwordSettingName = 'FLEX_CONTAINER_REGISTRY_PASSWORD'
        }
    }

    $patchBody = @{
        properties = @{
            functionAppConfig = $config
        }
    } | ConvertTo-Json -Depth 30

    $null = Invoke-RestMethod `
      -Method Patch `
      -Uri $siteUri `
      -Headers $headers `
      -ContentType 'application/json' `
      -Body ([Text.Encoding]::UTF8.GetBytes($patchBody))
}
finally {
    $accessToken = $null
    $headers = $null
    $credentialsJson = $null
    $credentials = $null
    $password = $null
    $settings = $null
    $settingsBody = $null
    $patchBody = $null
    $storageConnection = $null
    $appInsightsConnection = $null
}
```

Get a new ARM access token and verify exact readback:

```powershell
$accessToken = az account get-access-token `
  --subscription $subscription `
  --resource https://management.azure.com/ `
  --query accessToken `
  --output tsv

$headers = @{
    Authorization = [string]::Concat('Bearer ', $accessToken)
}
$live = Invoke-RestMethod -Method Get -Uri $siteUri -Headers $headers

if ($null -ne $live.properties.functionAppConfig.runtime) {
    throw 'Runtime was not cleared.'
}
$storageConfig = $live.properties.functionAppConfig.deployment.storage

if ($storageConfig.type -ine 'Registry') {
    throw 'Deployment type is not Registry.'
}
if ($storageConfig.value -cne $pinnedImage) {
    throw 'Pinned image readback does not match.'
}
if ($storageConfig.authentication.type -ine 'Basic') {
    throw 'Registry authentication readback does not match.'
}

$endpoint = "https://$($live.properties.defaultHostName)/api"
```

Renew the pull credential and update the app setting before the reported expiry.

## 8. Verify the endpoints

Use bounded retries because the first image pull and worker start can take time:

```powershell
function Invoke-BoundedCurl {
    param(
        [Parameter(Mandatory)]
        [string] $Uri,

        [Parameter(Mandatory)]
        [string] $OutFile,

        [string[]] $Arguments = @()
    )

    for ($attempt = 1; $attempt -le 30; $attempt++) {
        $status = & curl.exe `
          --silent `
          --show-error `
          --max-time 30 `
          @Arguments `
          --output $OutFile `
          --write-out '%{http_code}' `
          $Uri

        if ($LASTEXITCODE -eq 0 -and $status -eq '200') {
            return
        }

        if ($attempt -lt 30) {
            Start-Sleep -Seconds 10
        }
    }

    throw "$Uri did not return HTTP 200 after 30 attempts."
}
```

Verify the original Go function:

```powershell
$helloUri = "$endpoint/hello"

foreach ($method in 'GET', 'POST') {
    $helloFile = Join-Path $env:TEMP "go-container-hello-$method.txt"

    try {
        Invoke-BoundedCurl `
          -Uri $helloUri `
          -OutFile $helloFile `
          -Arguments @('--request', $method)

        $content = [IO.File]::ReadAllText($helloFile)
        if ($content -cne 'Hello from the Go worker.') {
            throw "$method /api/hello returned an unexpected body."
        }
    }
    finally {
        Remove-Item -Path $helloFile -ErrorAction SilentlyContinue
    }
}
```

Verify the built-in and custom PDFs:

```powershell
Invoke-BoundedCurl `
  -Uri "$endpoint/html-to-pdf" `
  -OutFile '.\invoice-get.pdf'

Invoke-BoundedCurl `
  -Uri "$endpoint/html-to-pdf" `
  -OutFile '.\invoice-post.pdf' `
  -Arguments @(
      '--request', 'POST',
      '--header', 'Content-Type: text/html; charset=utf-8',
      '--data-binary', '@examples/custom.html'
  )

foreach ($path in '.\invoice-get.pdf', '.\invoice-post.pdf') {
    $bytes = [IO.File]::ReadAllBytes($path)
    if ($bytes.Length -le 5 -or
        [Text.Encoding]::ASCII.GetString($bytes, 0, 5) -cne '%PDF-') {
        throw "$path is not a non-empty PDF."
    }
}
```

Open both files in a PDF viewer to complete the visual check.

## 9. Stop or clean up

Stop the app while retaining resources:

```powershell
az functionapp stop `
  --subscription $subscription `
  --resource-group $resourceGroup `
  --name $app

$site = Invoke-RestMethod -Method Get -Uri $siteUri -Headers $headers
if ($site.properties.state -ne 'Stopped') {
    throw "App state is $($site.properties.state), expected Stopped."
}
```

To permanently remove all resources created by this guide:

```powershell
az group delete `
  --subscription $subscription `
  --name $resourceGroup `
  --yes
```

The registry, repository, images, token, scope map, Function App, plan, storage,
and monitoring resources are all contained in this resource group.

## Known preview limitations

- The public Azure CLI rejects the internal Stage location for app creation.
- Image-local `host.json` forwarding is not established for this preview.
- The Go worker uses buffered HTTP over gRPC; HTTP streaming is not claimed.
- Warm-placeholder specialization is not established.
- The sample uses the Go worker's default HTTP request path, which preserved
  `Content-Type` during Azure validation. A forced or fallback HTTP-over-gRPC
  path remains affected by
  [Azure/azure-functions-golang-worker#87](https://github.com/Azure/azure-functions-golang-worker/issues/87),
  so the handler retains its missing-Content-Type compatibility behavior.
