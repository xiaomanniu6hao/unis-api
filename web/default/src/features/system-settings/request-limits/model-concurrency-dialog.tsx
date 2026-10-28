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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

const modelConcurrencyDialogSchema = z.object({
  modelName: z.string().min(1, 'Model name is required'),
  maxCount: z.number().min(1, 'Must be ≥ 1').max(1000, 'Must be ≤ 1000'),
})

type ModelConcurrencyDialogFormValues = z.infer<
  typeof modelConcurrencyDialogSchema
>

const MODEL_CONCURRENCY_FORM_ID = 'model-concurrency-form'

export type ModelConcurrencyEntryData = {
  modelName: string
  maxCount: number
}

type ModelConcurrencyDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (data: ModelConcurrencyEntryData) => void
  editData?: ModelConcurrencyEntryData | null
  modelOptions?: string[]
}

export function ModelConcurrencyDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  modelOptions = [],
}: ModelConcurrencyDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData

  const form = useForm<ModelConcurrencyDialogFormValues>({
    resolver: zodResolver(modelConcurrencyDialogSchema),
    defaultValues: {
      modelName: '',
      maxCount: 2,
    },
  })

  useEffect(() => {
    if (editData) {
      form.reset(editData)
    } else {
      form.reset({
        modelName: '',
        maxCount: 2,
      })
    }
  }, [editData, form, open])

  const handleSubmit = (values: ModelConcurrencyDialogFormValues) => {
    onSave(values)
    form.reset()
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        isEditMode
          ? t('Edit model concurrency override')
          : t('Add model concurrency override')
      }
      description={t('Set a concurrency limit for a specific model.')}
      contentClassName='sm:max-w-[500px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={MODEL_CONCURRENCY_FORM_ID}>
            {isEditMode ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={MODEL_CONCURRENCY_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='modelName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Model name')}</FormLabel>
                <FormControl>
                  <div className='space-y-1'>
                    <Input
                      placeholder={t('e.g., qwen3.5, glm5.1')}
                      list='model-concurrency-suggestions'
                      {...field}
                      disabled={isEditMode}
                    />
                    <datalist id='model-concurrency-suggestions'>
                      {modelOptions.map((model) => (
                        <option key={model} value={model} />
                      ))}
                    </datalist>
                  </div>
                </FormControl>
                <FormDescription>
                  {isEditMode
                    ? t('Model name cannot be changed when editing.')
                    : t('Start typing to see available models.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='maxCount'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max concurrent requests')}</FormLabel>
                <FormControl>
                  <div className='flex items-center gap-2'>
                    <Input
                      type='number'
                      min={1}
                      max={1000}
                      step={1}
                      {...field}
                      onChange={(e) =>
                        field.onChange(parseInt(e.target.value) || 1)
                      }
                    />
                    <span className='text-muted-foreground text-sm'>
                      {t('times')}
                    </span>
                  </div>
                </FormControl>
                <FormDescription>
                  {t(
                    'Maximum simultaneous in-flight requests for this model'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
