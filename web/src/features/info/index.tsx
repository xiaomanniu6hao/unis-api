/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Copy, Pencil, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PageTransition } from '@/components/page-transition'
import { Skeleton } from '@/components/ui/skeleton'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getNotice } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

import { getInfoTokens, updateNotice, type InfoToken } from './api'

// /info 公开页：展示 API 令牌（明文 key）与公告。移植自 MIXAPI /info。
// 令牌泄露为 MIXAPI 固有特性，由用户明确接受（见 memory: feedback-custom-port-approach）。
export function Info() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { copyToClipboard } = useCopyToClipboard()
  const [searchText, setSearchText] = useState('')
  const [isEditNotice, setIsEditNotice] = useState(false)
  const [noticeInput, setNoticeInput] = useState('')

  const tokensQuery = useQuery({
    queryKey: ['info-tokens'],
    queryFn: () => getInfoTokens(),
    staleTime: 60 * 1000,
  })
  const noticeQuery = useQuery({
    queryKey: ['notice'],
    queryFn: getNotice,
    staleTime: 60 * 1000,
  })

  // root 用户才能编辑公告。localStorage 的 user 由登录流程写入。
  const isRoot = useMemo(() => {
    try {
      const raw = localStorage.getItem('user')
      if (!raw) return false
      const user = JSON.parse(raw)
      return user?.role === 100 || user?.username === 'root'
    } catch {
      return false
    }
  }, [])

  const notice = noticeQuery.data?.data ?? ''

  const noticeMutation = useMutation({
    mutationFn: (value: string) => updateNotice(value),
    onSuccess: (data) => {
      if (data.success) {
        queryClient.invalidateQueries({ queryKey: ['notice'] })
        setIsEditNotice(false)
      } else {
        handleServerError(new Error(data.message || t('Save failed')))
      }
    },
    onError: (err) => handleServerError(err),
  })

  const allTokens = tokensQuery.data?.data?.items ?? []
  const filteredTokens = useMemo(() => {
    if (!searchText.trim()) return allTokens
    const keyword = searchText.toLowerCase()
    return allTokens.filter(
      (token) =>
        token.name?.toLowerCase().includes(keyword) ||
        token.key?.toLowerCase().includes(keyword) ||
        token.group?.toLowerCase().includes(keyword)
    )
  }, [searchText, allTokens])

  const handleCopyKey = (key: string) => {
    copyToClipboard(`sk-${key}`)
  }

  const handleSaveNotice = () => {
    noticeMutation.mutate(noticeInput)
  }

  const openEditNotice = () => {
    setNoticeInput(notice)
    setIsEditNotice(true)
  }

  return (
    <PageTransition className='mx-auto w-full max-w-6xl space-y-6 px-4 py-8 sm:px-6'>
      <h1 className='text-center text-2xl font-bold'>{t('API Information')}</h1>

      {/* 公告卡片 */}
      <section className='bg-card rounded-xl border p-6 shadow-sm'>
        <div className='mb-3 flex items-center justify-between'>
          <span className='text-muted-foreground text-sm'>{t('Notice')}</span>
          {isRoot && (
            <button
              type='button'
              onClick={openEditNotice}
              className='text-primary inline-flex items-center gap-1 text-sm hover:underline'
            >
              <Pencil className='h-3.5 w-3.5' />
              {t('Edit Notice')}
            </button>
          )}
        </div>
        {noticeQuery.isLoading ? (
          <Skeleton className='h-6 w-3/4' />
        ) : (
          <div className='whitespace-pre-wrap text-sm'>
            {notice || t('No notice')}
          </div>
        )}
      </section>

      {/* 令牌卡片 */}
      <section className='bg-card rounded-xl border p-6 shadow-sm'>
        <div className='mb-4 flex items-center justify-between gap-4'>
          <h2 className='m-0 text-base font-semibold'>{t('API Tokens')}</h2>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
            <input
              type='text'
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              placeholder={t('Search tokens...')}
              className='bg-background border-input focus-visible:ring-ring h-9 w-56 rounded-md border pr-3 pl-8 text-sm outline-none focus-visible:ring-2'
            />
          </div>
        </div>

        {tokensQuery.isLoading ? (
          <Skeleton className='h-64 w-full' />
        ) : (
          <div className='overflow-x-auto'>
            <table className='w-full text-sm'>
              <thead>
                <tr className='text-muted-foreground border-b text-left'>
                  <th className='py-2 pr-4 font-medium'>{t('Name')}</th>
                  <th className='py-2 pr-4 font-medium'>{t('Key')}</th>
                  <th className='py-2 pr-4 font-medium'>{t('Group')}</th>
                  <th className='py-2 font-medium'>{t('Action')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredTokens.length === 0 ? (
                  <tr>
                    <td
                      colSpan={4}
                      className='text-muted-foreground py-8 text-center'
                    >
                      {t('No tokens')}
                    </td>
                  </tr>
                ) : (
                  filteredTokens.map((token) => (
                    <TokenRow
                      key={token.id}
                      token={token}
                      onCopy={handleCopyKey}
                    />
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* 编辑公告弹层 */}
      {isEditNotice && (
        <div
          className='fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4'
          onClick={() => setIsEditNotice(false)}
        >
          <div
            className='bg-card w-full max-w-lg rounded-xl border p-6 shadow-lg'
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className='mb-3 text-base font-semibold'>{t('Edit Notice')}</h3>
            <textarea
              value={noticeInput}
              onChange={(e) => setNoticeInput(e.target.value)}
              placeholder={t('Enter notice content')}
              rows={6}
              className='bg-background border-input focus-visible:ring-ring w-full rounded-md border p-2 text-sm outline-none focus-visible:ring-2'
            />
            <div className='mt-4 flex justify-end gap-2'>
              <button
                type='button'
                onClick={() => setIsEditNotice(false)}
                className='border-input hover:bg-accent rounded-md border px-4 py-1.5 text-sm'
              >
                {t('Cancel')}
              </button>
              <button
                type='button'
                onClick={handleSaveNotice}
                disabled={noticeMutation.isPending}
                className='bg-primary text-primary-foreground hover:bg-primary/90 rounded-md px-4 py-1.5 text-sm disabled:opacity-50'
              >
                {t('Save')}
              </button>
            </div>
          </div>
        </div>
      )}
    </PageTransition>
  )
}

function TokenRow({
  token,
  onCopy,
}: {
  token: InfoToken
  onCopy: (key: string) => void
}) {
  const { t } = useTranslation()
  return (
    <tr className='border-b last:border-0'>
      <td className='py-2 pr-4'>{token.name}</td>
      <td className='py-2 pr-4'>
        <span className='inline-flex items-center gap-1'>
          <span className='text-muted-foreground'>sk-</span>
          <code className='bg-muted rounded px-1.5 py-0.5 font-mono text-xs'>
            {token.key}
          </code>
        </span>
      </td>
      <td className='text-muted-foreground py-2 pr-4'>{token.group || '-'}</td>
      <td className='py-2'>
        <button
          type='button'
          onClick={() => onCopy(token.key)}
          className='hover:bg-accent inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs'
        >
          <Copy className='h-3.5 w-3.5' />
          {t('Copy Key')}
        </button>
      </td>
    </tr>
  )
}
