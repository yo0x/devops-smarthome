function New-AESKey {
    param (
        [Parameter(Mandatory=$true)]
        [string]$KeyFile
    )
    
    try {
        # Generate a new 256-bit key
        $AESKey = New-Object Byte[] 32
        [Security.Cryptography.RNGCryptoServiceProvider]::Create().GetBytes($AESKey)
        
        # Save the key as base64 string
        [System.Convert]::ToBase64String($AESKey) | Set-Content $KeyFile
        
        Write-Host "AES-256 key generated and saved to: $KeyFile"
    }
    catch {
        Write-Error "Key generation failed: $_"
    }
}

function Protect-FileWithAES {
    param (
        [Parameter(Mandatory=$true)]
        [string]$InputFile,
        
        [Parameter(Mandatory=$true)]
        [string]$OutputFile,
        
        [Parameter(Mandatory=$true)]
        [string]$KeyFile
    )
    
    try {
        # Check if key exists or generate new one
        if (-not (Test-Path $KeyFile)) {
            New-AESKey -KeyFile $KeyFile
        }
        
        # Read the key
        $KeyBytes = [System.Convert]::FromBase64String((Get-Content $KeyFile))
        
        # Create AES object
        $AES = New-Object System.Security.Cryptography.AesManaged
        $AES.KeySize = 256
        $AES.Key = $KeyBytes
        $AES.GenerateIV() # Generate random IV
        
        # Read the input file
        $plainBytes = [System.Text.Encoding]::UTF8.GetBytes((Get-Content $InputFile -Raw))
        
        # Create encryptor
        $encryptor = $AES.CreateEncryptor()
        
        # Encrypt the data
        $encryptedBytes = $encryptor.TransformFinalBlock($plainBytes, 0, $plainBytes.Length)
        
        # Combine IV and encrypted data
        $result = $AES.IV + $encryptedBytes
        
        # Convert to Base64 and save
        [System.Convert]::ToBase64String($result) | Set-Content $OutputFile
        
        Write-Host "File encrypted successfully: $OutputFile"
        Write-Host "Keep the key file safe: $KeyFile"
    }
    catch {
        Write-Error "Encryption failed: $_"
    }
    finally {
        if ($AES) { $AES.Dispose() }
        if ($encryptor) { $encryptor.Dispose() }
    }
}

function Unprotect-FileWithAES {
    param (
        [Parameter(Mandatory=$true)]
        [string]$InputFile,
        
        [Parameter(Mandatory=$true)]
        [string]$OutputFile,
        
        [Parameter(Mandatory=$true)]
        [string]$KeyFile
    )
    
    try {
        # Verify files exist
        if (-not (Test-Path $InputFile)) { throw "Encrypted file not found" }
        if (-not (Test-Path $KeyFile)) { throw "Key file not found" }
        
        # Read the key
        $KeyBytes = [System.Convert]::FromBase64String((Get-Content $KeyFile))
        
        # Read and decode the encrypted data
        $combinedBytes = [System.Convert]::FromBase64String((Get-Content $InputFile))
        
        # Create AES object
        $AES = New-Object System.Security.Cryptography.AesManaged
        $AES.KeySize = 256
        $AES.Key = $KeyBytes
        
        # Extract IV (first 16 bytes) and encrypted data
        $IV = $combinedBytes[0..15]
        $encryptedBytes = $combinedBytes[16..$combinedBytes.Length]
        $AES.IV = $IV
        
        # Create decryptor
        $decryptor = $AES.CreateDecryptor()
        
        # Decrypt the data
        $decryptedBytes = $decryptor.TransformFinalBlock($encryptedBytes, 0, $encryptedBytes.Length)
        $decryptedText = [System.Text.Encoding]::UTF8.GetString($decryptedBytes)
        
        # Save decrypted content
        $decryptedText | Set-Content $OutputFile
        
        Write-Host "File decrypted successfully: $OutputFile"
    }
    catch {
        Write-Error "Decryption failed: $_"
    }
    finally {
        if ($AES) { $AES.Dispose() }
        if ($decryptor) { $decryptor.Dispose() }
    }
}

function Test-AESKey {
    param (
        [Parameter(Mandatory=$true)]
        [string]$KeyFile
    )
    
    try {
        if (-not (Test-Path $KeyFile)) {
            Write-Error "Key file not found"
            return $false
        }
        
        $keyContent = Get-Content $KeyFile
        $keyBytes = [System.Convert]::FromBase64String($keyContent)
        
        if ($keyBytes.Length -eq 32) {
            Write-Host "Valid AES-256 key found in: $KeyFile"
            return $true
        } else {
            Write-Error "Invalid key length. Expected 32 bytes for AES-256"
            return $false
        }
    }
    catch {
        Write-Error "Invalid key format: $_"
        return $false
    }
}