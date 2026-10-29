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
import { useEffect, useState } from 'react'

import { useTheme } from '@/context/theme-provider'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

// VChart 的主题资源是全局的，必须先注册一次再渲染图表；
// 返回 true 表示当前主题已经就绪，可以直接挂载 VChart。
export function useVChartThemeReady(): boolean {
  const { resolvedTheme } = useTheme()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let cancelled = false
    const updateTheme = async () => {
      setReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      if (cancelled) return
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setReady(true)
    }
    void updateTheme()
    return () => {
      cancelled = true
    }
  }, [resolvedTheme])

  return ready
}
