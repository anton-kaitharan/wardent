param([string]$In, [string]$Out)
$items = Get-Content -Raw -Encoding UTF8 $In | ConvertFrom-Json
$res = @()
foreach ($it in $items) {
  $tokens = $null; $errors = $null
  $ast = [System.Management.Automation.Language.Parser]::ParseInput($it.cmd, [ref]$tokens, [ref]$errors)
  $names = @()
  $cmds = $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.CommandAst] }, $true)
  foreach ($c in $cmds) {
    $n = $c.GetCommandName()
    if ($null -eq $n) { $names += '<dyn>'; continue }
    $a = Get-Alias -Name $n -ErrorAction SilentlyContinue
    if ($a) { $n = $a.Definition }
    $names += $n.ToLower()
  }
  $res += [pscustomobject]@{ id = $it.id; ok = ($errors.Count -eq 0); exes = @($names) }
}
ConvertTo-Json -InputObject @($res) -Depth 4 | Out-File -Encoding UTF8 $Out
