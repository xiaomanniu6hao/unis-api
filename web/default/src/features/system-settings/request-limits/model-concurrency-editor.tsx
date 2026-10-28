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
import { Plus, Search } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { useEnabledModels } from '../hooks/use-enabled-models'
import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isObjectRecord } from '../utils/json-validators'
import {
  ModelConcurrencyDialog,
  type ModelConcurrencyEntryData,
} from './model-concurrency-dialog'

type ModelConcurrencyEditorProps = {
  value: string
  onChange: (value: string) => void
}

export function ModelConcurrencyEditor({
  value,
  onChange,
}: ModelConcurrencyEditorProps) {
  const { t } = useTranslation()
  const [searchText, setSearchText] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<ModelConcurrencyEntryData | null>(
    null
  )

  const modelsQuery = useEnabledModels()

  const overrides = useMemo(() => {
    if (!value || value.trim() === '') return []

    const parsed = safeJsonParseWithValidation<Record<string, unknown>>(value, {
      fallback: {},
      validator: isObjectRecord,
      validatorMessage: 'Model overrides must be a JSON object',
      context: 'model overrides',
    })

    return Object.entries(parsed)
      .map(([modelName, maxCount]) => {
        if (typeof maxCount === 'number' && Number.isInteger(maxCount)) {
          return { modelName, maxCount }
        }
        return null
      })
      .filter((item): item is ModelConcurrencyEntryData => item !== null)
  }, [value])

  const filteredOverrides = useMemo(() => {
    if (!searchText) return overrides
    const lowerSearch = searchText.toLowerCase()
    return overrides.filter((override) =>
      override.modelName.toLowerCase().includes(lowerSearch)
    )
  }, [overrides, searchText])

  const handleSave = (data: ModelConcurrencyEntryData) => {
    const parsed = safeJsonParseWithValidation<Record<string, unknown>>(value, {
      fallback: {},
      validator: isObjectRecord,
      silent: true,
    })

    if (editData && editData.modelName !== data.modelName) {
      delete parsed[editData.modelName]
    }

    parsed[data.modelName] = data.maxCount

    onChange(JSON.stringify(parsed, null, 2))
  }

  const handleDelete = (modelName: string) => {
    const parsed = safeJsonParseWithValidation<Record<string, unknown>>(value, {
      fallback: {},
      validator: isObjectRecord,
      silent: true,
    })

    delete parsed[modelName]

    onChange(JSON.stringify(parsed, null, 2))
  }

  const handleEdit = (override: ModelConcurrencyEntryData) => {
    setEditData(override)
    setDialogOpen(true)
  }

  const handleAdd = () => {
    setEditData(null)
    setDialogOpen(true)
  }

  return (
    <div className='space-y-4'>
      <div className='flex items-center gap-4'>
        <div className='relative flex-1'>
          <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
          <Input
            placeholder={t('Search models...')}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            className='pl-9'
          />
        </div>
        <Button onClick={handleAdd}>
          <Plus className='mr-2 h-4 w-4' />
          {t('Add model')}
        </Button>
      </div>

      <StaticDataTable
        data={filteredOverrides}
        getRowKey={(override) => override.modelName}
        emptyContent={
          searchText
            ? t('No models match your search')
            : t(
                'No model overrides configured. Click "Add model" to get started.'
              )
        }
        columns={[
          {
            id: 'model',
            header: t('Model name'),
            cellClassName: 'font-medium',
            cell: (override) => override.modelName,
          },
          {
            id: 'max-count',
            header: t('Max concurrent requests'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (override) => (
              <span className='font-mono'>
                {override.maxCount.toLocaleString()}
              </span>
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (override) => (
              <StaticRowActions
                editLabel={t('Edit')}
                deleteLabel={t('Delete')}
                menuLabel={t('Open menu')}
                onEdit={() => handleEdit(override)}
                onDelete={() => handleDelete(override.modelName)}
              />
            ),
          },
        ]}
      />

      <ModelConcurrencyDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
        modelOptions={modelsQuery.data ?? []}
      />
    </div>
  )
}
