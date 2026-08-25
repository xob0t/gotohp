<script setup lang="ts">
import { ref } from 'vue'
import { Browser } from '@wailsio/runtime'
import { toast } from 'vue-sonner'
import { ConfigManager } from '../../bindings/app/backend'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

const embeddedSetupURL = 'https://accounts.google.com/EmbeddedSetup'

const emit = defineEmits<{
  (event: 'account-added'): void
}>()

const isOpen = ref(false)
const oauthToken = ref('')
const rawCredential = ref('')
const isConnecting = ref(false)
const isAddingRawCredential = ref(false)

async function openEmbeddedSetup() {
  try {
    await Browser.OpenURL(embeddedSetupURL)
  } catch (error) {
    toast.error('Could not open the system browser', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}

async function connectAccount() {
  const normalizedToken = oauthToken.value.trim()
  if (!normalizedToken) return

  isConnecting.value = true
  try {
    const connectedEmail = await ConfigManager.AddGoogleAccount(normalizedToken)
    oauthToken.value = ''
    isOpen.value = false
    emit('account-added')
    toast.success('Google account connected.', {
      description: connectedEmail,
    })
  } catch (error) {
    toast.error('Could not connect Google account', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    oauthToken.value = ''
    isConnecting.value = false
  }
}

async function addRawCredential() {
  const credential = rawCredential.value.trim()
  if (!credential) return

  isAddingRawCredential.value = true
  try {
    await ConfigManager.AddCredentials(credential)
    rawCredential.value = ''
    isOpen.value = false
    emit('account-added')
    toast.success('Credentials added.')
  } catch (error) {
    toast.error('Could not add credentials', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isAddingRawCredential.value = false
  }
}
</script>

<template>
  <Sheet v-model:open="isOpen">
    <SheetTrigger as-child>
      <Button
        type="button"
        class="cursor-pointer select-none"
      >
        Add Google account
      </Button>
    </SheetTrigger>
    <SheetContent
      side="bottom"
      class="max-h-[96vh] overflow-y-auto"
      style="--wails-draggable: none"
    >
      <SheetHeader class="gap-1 pb-2">
        <SheetTitle>Connect Google Photos</SheetTitle>
        <SheetDescription>
          Sign in with the account you want, then paste its one-time cookie value.
        </SheetDescription>
      </SheetHeader>

      <div class="flex flex-col gap-3 px-4 pb-3">
        <Button
          type="button"
          variant="outline"
          class="cursor-pointer select-none"
          :disabled="isConnecting"
          @click="openEmbeddedSetup"
        >
          Open Google sign-in
        </Button>

        <div class="rounded-lg border p-2.5 text-[11px] leading-snug text-muted-foreground">
          <ol class="list-decimal space-y-1 pl-4">
            <li>Sign in with the account you want, then click <span class="text-foreground">I agree</span>.</li>
            <li>In DevTools, open Application or Storage → Cookies → accounts.google.com.</li>
            <li>Copy only the <code class="text-foreground">oauth_token</code> cookie value.</li>
          </ol>
        </div>

        <div class="flex flex-col gap-1.5">
          <Label for="google-oauth-token">oauth_token value</Label>
          <Input
            id="google-oauth-token"
            v-model="oauthToken"
            type="password"
            autocomplete="off"
            spellcheck="false"
            placeholder="Paste the cookie value"
            :disabled="isConnecting"
            @keydown.enter="connectAccount"
          />
          <p class="text-[11px] leading-snug text-muted-foreground">
            Exchanged once, never saved, and usually cannot be reused.
          </p>
        </div>

        <Button
          type="button"
          class="cursor-pointer select-none"
          :disabled="!oauthToken.trim() || isConnecting"
          @click="connectAccount"
        >
          {{ isConnecting ? 'Connecting...' : 'Connect account' }}
        </Button>

        <details class="rounded-lg border p-2.5 text-sm">
          <summary class="cursor-pointer select-none text-muted-foreground">
            Advanced: paste captured credentials
          </summary>
          <div class="mt-3 flex flex-col gap-2">
            <Input
              v-model="rawCredential"
              type="password"
              autocomplete="off"
              spellcheck="false"
              placeholder="androidId=...&Email=..."
              :disabled="isAddingRawCredential"
              @keydown.enter="addRawCredential"
            />
            <Button
              type="button"
              variant="outline"
              class="cursor-pointer select-none"
              :disabled="!rawCredential.trim() || isAddingRawCredential"
              @click="addRawCredential"
            >
              {{ isAddingRawCredential ? 'Adding...' : 'Add captured credentials' }}
            </Button>
          </div>
        </details>
      </div>
    </SheetContent>
  </Sheet>
</template>
