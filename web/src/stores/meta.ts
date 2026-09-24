import { defineStore } from 'pinia'
import { ref } from 'vue'
import { metaApi } from '@/api'
import type { TypeMeta } from '@/api/types'

/** 服务商 / 通知渠道的字段 Schema 缓存 */
export const useMetaStore = defineStore('meta', () => {
  const providers = ref<TypeMeta[]>([])
  const notifiers = ref<TypeMeta[]>([])
  let providersLoaded: Promise<void> | null = null
  let notifiersLoaded: Promise<void> | null = null

  function loadProviders(force = false) {
    if (!providersLoaded || force) {
      providersLoaded = metaApi
        .providers()
        .then((r) => {
          providers.value = r ?? []
        })
        .catch((e) => {
          providersLoaded = null
          throw e
        })
    }
    return providersLoaded
  }

  function loadNotifiers(force = false) {
    if (!notifiersLoaded || force) {
      notifiersLoaded = metaApi
        .notifiers()
        .then((r) => {
          notifiers.value = r ?? []
        })
        .catch((e) => {
          notifiersLoaded = null
          throw e
        })
    }
    return notifiersLoaded
  }

  const providerName = (type: string) => providers.value.find((p) => p.type === type)?.name ?? type
  const notifierName = (type: string) => notifiers.value.find((p) => p.type === type)?.name ?? type

  return { providers, notifiers, loadProviders, loadNotifiers, providerName, notifierName }
})
