param(
    [Parameter(Position=0)]
    [ValidateSet('build','run','test','cert','dns-test','acme-test','clean')]
    [string]$Target = 'build'
)

function Invoke-Build {
    go build .
}

function Invoke-Run {
    go run .
}

function Invoke-Test {
    go test ./...
}

function Invoke-Cert {
    Invoke-Build
    .\libdns-websupport.exe create-cert
}

function Invoke-DnsTest {
    Invoke-Build
    .\libdns-websupport.exe test
}

function Invoke-AcmeTest {
    Invoke-Build
    .\libdns-websupport.exe acme-test
}

function Invoke-Clean {
    if (Test-Path .\libdns-websupport.exe) { Remove-Item .\libdns-websupport.exe -Force }
}

switch ($Target) {
    'build'     { Invoke-Build }
    'run'       { Invoke-Run }
    'test'      { Invoke-Test }
    'cert'      { Invoke-Cert }
    'dns-test'  { Invoke-DnsTest }
    'acme-test' { Invoke-AcmeTest }
    'clean'     { Invoke-Clean }
}
