$ErrorActionPreference = 'Stop'

function Test-ExactAccessControl([System.Security.AccessControl.FileSystemSecurity]$security, [System.Security.Principal.SecurityIdentifier]$sid) {
  if (-not $security.AreAccessRulesProtected -or $security.GetOwner([System.Security.Principal.SecurityIdentifier]).Value -ne $sid.Value) {
    return $false
  }

  $rules = $security.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])
  if ($rules.Count -ne 1) {
    return $false
  }
  $rule = $rules[0]
  return -not $rule.IsInherited -and
    $rule.IdentityReference.Value -eq $sid.Value -and
    $rule.AccessControlType -eq [System.Security.AccessControl.AccessControlType]::Allow -and
    $rule.FileSystemRights -eq [System.Security.AccessControl.FileSystemRights]::FullControl
}

function Test-ExactDirectory([string]$path, [System.Security.Principal.SecurityIdentifier]$sid) {
  $item = Get-Item -LiteralPath $path -Force
  if (-not $item.PSIsContainer -or ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
    return $false
  }
  return Test-ExactAccessControl $item.GetAccessControl() $sid
}

try {
  [Console]::Out.Write("PROCESS_ENTRY`n")
  [Console]::Out.Flush()
  $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
  $root = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
  if ([string]::IsNullOrWhiteSpace($root)) {
    throw 'LocalApplicationData is unavailable'
  }
  $directoryPath = Join-Path $root 'BAC Nexus\companion-v1'

  $directorySecurity = New-Object System.Security.AccessControl.DirectorySecurity
  $directorySecurity.SetOwner($sid)
  $directorySecurity.SetAccessRuleProtection($true, $false)
  $directorySecurity.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule(
    $sid,
    [System.Security.AccessControl.FileSystemRights]::FullControl,
    [System.Security.AccessControl.InheritanceFlags]'ContainerInherit, ObjectInherit',
    [System.Security.AccessControl.PropagationFlags]::None,
    [System.Security.AccessControl.AccessControlType]::Allow
  )))
  [void][System.IO.Directory]::CreateDirectory($directoryPath, $directorySecurity)
  if (-not (Test-ExactDirectory $directoryPath $sid)) {
    throw 'descriptor directory security is not exact'
  }

  [Console]::Out.Write("DIRECTORY_VALIDATED`n")
  [Console]::Out.Flush()
  [Console]::Out.Write("READY`n")
  [Console]::Out.Flush()
  $input = [Console]::In.ReadToEnd()
  $inputBytes = [System.Text.Encoding]::UTF8.GetBytes($input)
  if ($inputBytes.Length -eq 0 -or $inputBytes.Length -gt 512) {
    throw 'descriptor input is out of bounds'
  }
  $descriptor = $input | ConvertFrom-Json
  if ($descriptor.PSObject.Properties.Count -ne 3 -or
      $descriptor.version -ne 1 -or
      $descriptor.generation -notmatch '^[0-9a-f]{32}$' -or
      $descriptor.token -notmatch '^[0-9a-f]{64}$') {
    throw 'descriptor input is invalid'
  }

  $descriptorPath = Join-Path $directoryPath 'descriptor.json'
  $fileSecurity = New-Object System.Security.AccessControl.FileSecurity
  $fileSecurity.SetOwner($sid)
  $fileSecurity.SetAccessRuleProtection($true, $false)
  $fileSecurity.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule(
    $sid,
    [System.Security.AccessControl.FileSystemRights]::FullControl,
    [System.Security.AccessControl.AccessControlType]::Allow
  )))
  $stream = New-Object System.IO.FileStream(
    $descriptorPath,
    [System.IO.FileMode]::CreateNew,
    [System.Security.AccessControl.FileSystemRights]::FullControl,
    [System.IO.FileShare]::None,
    4096,
    [System.IO.FileOptions]::None,
    $fileSecurity
  )
  try {
    $fileItem = Get-Item -LiteralPath $descriptorPath -Force
    if ($fileItem.PSIsContainer -or
        ($fileItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -or
        $stream.SafeFileHandle.IsInvalid -or
        -not (Test-ExactAccessControl $stream.GetAccessControl() $sid)) {
      throw 'descriptor file security is not exact'
    }
    $stream.Write($inputBytes, 0, $inputBytes.Length)
    $stream.Flush($true)
  } finally {
    $stream.Dispose()
  }
} catch {
  exit 1
}
