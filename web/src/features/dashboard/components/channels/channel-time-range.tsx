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
import { CalendarRange } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  type ChannelRangePreset,
  type ChannelTimeRange,
  resolveChannelTimeRange,
} from '@/features/dashboard/lib/channel-range'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

const PRESETS: Array<{ preset: ChannelRangePreset; label: string }> = [
  { preset: 'day', label: 'Today' },
  { preset: 'week', label: 'This week' },
  { preset: 'month', label: 'This month' },
]

interface Props {
  value: ChannelTimeRange
  onChange: (range: ChannelTimeRange) => void
}

export function ChannelTimeRangePicker(props: Props) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [start, setStart] = useState<Date | undefined>(props.value.start)
  const [end, setEnd] = useState<Date | undefined>(props.value.end)

  const handlePreset = (preset: ChannelRangePreset) => {
    props.onChange(resolveChannelTimeRange(preset))
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      setStart(props.value.start)
      setEnd(props.value.end)
    }
    setOpen(nextOpen)
  }

  const handleApply = () => {
    props.onChange(resolveChannelTimeRange('custom', { start, end }))
    setOpen(false)
  }

  const rangeLabel = `${dayjs(props.value.start).format('MM-DD HH:mm')} ~ ${dayjs(
    props.value.end
  ).format('MM-DD HH:mm')}`

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <div className='bg-muted/60 flex items-center gap-1 rounded-md p-0.5'>
        {PRESETS.map((item) => (
          <button
            key={item.preset}
            type='button'
            onClick={() => handlePreset(item.preset)}
            className={cn(
              'rounded px-2 py-1 text-xs transition-colors',
              props.value.preset === item.preset
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {t(item.label)}
          </button>
        ))}
        <Dialog
          open={open}
          onOpenChange={handleOpenChange}
          title={t('Custom time range')}
          description={t('Pick the start and end time of the analysis window.')}
          trigger={
            <Button
              variant='ghost'
              size='sm'
              className={cn(
                'h-auto rounded px-2 py-1 text-xs',
                props.value.preset === 'custom'
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <CalendarRange className='mr-1 size-3.5' />
              {t('Custom')}
            </Button>
          }
          footer={
            <>
              <Button variant='outline' onClick={() => setOpen(false)}>
                {t('Cancel')}
              </Button>
              <Button onClick={handleApply}>{t('Apply')}</Button>
            </>
          }
        >
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='grid gap-1.5'>
              <Label>{t('Start time')}</Label>
              <DateTimePicker value={start} onChange={setStart} />
            </div>
            <div className='grid gap-1.5'>
              <Label>{t('End time')}</Label>
              <DateTimePicker value={end} onChange={setEnd} />
            </div>
          </div>
        </Dialog>
      </div>
      <span className='text-muted-foreground font-mono text-[11px]'>
        {rangeLabel}
      </span>
    </div>
  )
}
