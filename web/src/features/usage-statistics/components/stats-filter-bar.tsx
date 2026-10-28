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
import { Download, RotateCcw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { StatsTokenOption } from '../api'

export type StatsFilterValues = {
  startDate: string
  endDate: string
  tokenId: number
  modelName: string
  groupBy: string
}

type StatsFilterBarProps = {
  value: StatsFilterValues
  onChange: (next: StatsFilterValues) => void
  onSearch: () => void
  onReset: () => void
  onExport: () => void
  exporting?: boolean
  tokens: StatsTokenOption[]
  showGroupBy?: boolean
  showTokenFilter?: boolean
  showPercentToggle?: boolean
  percent?: boolean
  onTogglePercent?: (next: boolean) => void
}

// 统计页共享筛选条：日期范围 + 令牌 + 模型名 +（可选）分组方式 +（可选）占比开关。
export function StatsFilterBar({
  value,
  onChange,
  onSearch,
  onReset,
  onExport,
  exporting,
  tokens,
  showGroupBy = false,
  showTokenFilter = true,
  showPercentToggle = false,
  percent = false,
  onTogglePercent,
}: StatsFilterBarProps) {
  const { t } = useTranslation()

  return (
    <div className='bg-card flex flex-wrap items-end gap-3 rounded-lg border p-4'>
      <div className='flex flex-col gap-1'>
        <label className='text-muted-foreground text-xs'>
          {t('Start Date')}
        </label>
        <Input
          type='date'
          value={value.startDate}
          onChange={(e) => onChange({ ...value, startDate: e.target.value })}
          className='h-8 w-[150px]'
        />
      </div>
      <div className='flex flex-col gap-1'>
        <label className='text-muted-foreground text-xs'>{t('End Date')}</label>
        <Input
          type='date'
          value={value.endDate}
          onChange={(e) => onChange({ ...value, endDate: e.target.value })}
          className='h-8 w-[150px]'
        />
      </div>
      {showTokenFilter && (
        <div className='flex flex-col gap-1'>
          <label className='text-muted-foreground text-xs'>{t('Token')}</label>
          <Select
            value={value.tokenId ? String(value.tokenId) : '0'}
            onValueChange={(v) =>
              onChange({ ...value, tokenId: Number(v) })
            }
          >
            <SelectTrigger className='h-8 w-[180px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='0'>{t('All Tokens')}</SelectItem>
              {tokens.map((token) => (
                <SelectItem key={token.id} value={String(token.id)}>
                  {token.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      <div className='flex flex-col gap-1'>
        <label className='text-muted-foreground text-xs'>
          {t('Model Name')}
        </label>
        <Input
          value={value.modelName}
          onChange={(e) => onChange({ ...value, modelName: e.target.value })}
          placeholder={t('Model keyword')}
          className='h-8 w-[180px]'
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSearch()
          }}
        />
      </div>
      {showGroupBy && (
        <div className='flex flex-col gap-1'>
          <label className='text-muted-foreground text-xs'>
            {t('Group By')}
          </label>
          <Select
            value={value.groupBy}
            onValueChange={(v) => onChange({ ...value, groupBy: v })}
          >
            <SelectTrigger className='h-8 w-[140px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='prefix'>{t('By Prefix')}</SelectItem>
              <SelectItem value='exact'>{t('By Exact Token')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}
      <div className='flex items-center gap-2'>
        <Button size='sm' onClick={onSearch}>
          <Search className='size-3.5' />
          {t('Search')}
        </Button>
        <Button size='sm' variant='outline' onClick={onReset}>
          <RotateCcw className='size-3.5' />
          {t('Reset')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={onExport}
          disabled={exporting}
        >
          <Download className='size-3.5' />
          {exporting ? t('Exporting...') : t('Export CSV')}
        </Button>
      </div>
      {showPercentToggle && (
        <div className='flex items-center gap-2'>
          <label className='text-muted-foreground flex items-center gap-2 text-xs'>
            {t('Show Percentage')}
            <Switch checked={percent} onCheckedChange={onTogglePercent} />
          </label>
        </div>
      )}
    </div>
  )
}
