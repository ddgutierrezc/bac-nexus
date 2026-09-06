$ErrorActionPreference = 'Stop'

try {
  $input = [Console]::In.ReadToEnd()
  $inputBytes = [System.Text.Encoding]::UTF8.GetBytes($input)
  if ($inputBytes.Length -eq 0 -or $inputBytes.Length -gt 128) {
    throw 'cleanup input is out of bounds'
  }
  $request = $input | ConvertFrom-Json
  if ($request.PSObject.Properties.Count -ne 1 -or $request.generation -notmatch '^[0-9a-f]{32}$') {
    throw 'cleanup input is invalid'
  }

  $root = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
  $descriptorPath = Join-Path $root 'BAC Nexus\companion-v1\descriptor.json'
  $descriptor = Get-Content -LiteralPath $descriptorPath -Raw | ConvertFrom-Json
  if ($descriptor.generation -ne $request.generation) {
    throw 'descriptor generation is not owned'
  }
  Remove-Item -LiteralPath $descriptorPath -Force -ErrorAction Stop
  [Console]::Out.Write("CLEANED`n")
} catch {
  exit 1
}
