import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BackupView from '../BackupView.vue'

const {
  getS3Config,
  getImageStorageConfig,
  getSchedule,
  updateSchedule,
  deleteBackup,
  listBackups,
  getDownloadURL,
  getBackup,
  createBackup,
  restoreBackup,
} = vi.hoisted(() => ({
  getS3Config: vi.fn(),
  getImageStorageConfig: vi.fn(),
  getSchedule: vi.fn(),
  updateSchedule: vi.fn(),
  deleteBackup: vi.fn(),
  listBackups: vi.fn(),
  getDownloadURL: vi.fn(),
  getBackup: vi.fn(),
  createBackup: vi.fn(),
  restoreBackup: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    backup: {
      getS3Config,
      updateS3Config: vi.fn(),
      testS3Connection: vi.fn(),
      getImageStorageConfig,
      updateImageStorageConfig: vi.fn(),
      testImageStorageConnection: vi.fn(),
      getSchedule,
      updateSchedule,
      createBackup,
      listBackups,
      getBackup,
      deleteBackup,
      getDownloadURL,
      restoreBackup,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: (fn: () => unknown) => fn() }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.index !== undefined ? `${key}:${params.index}` : params?.day !== undefined ? `${key}:${params.day}` : key,
  }),
}))

const baseRecord = (id: string, parts?: unknown[]) => ({
  id,
  status: 'completed',
  backup_type: 'postgres',
  file_name: `${id}.sql.gz`,
  s3_key: `backups/${id}.sql.gz`,
  parts,
  size_bytes: 10,
  triggered_by: 'manual',
  started_at: '2026-08-09T00:00:00Z',
})

const wrappers: ReturnType<typeof mount>[] = []

function mountBackupView() {
  const wrapper = mount(BackupView, {
    global: {
      stubs: {
        TotpStepUpDialog: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('admin BackupView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getS3Config.mockResolvedValue({})
    getImageStorageConfig.mockResolvedValue({ config: {}, secret_configured: false })
    getSchedule.mockResolvedValue({ enabled: false, cron_expr: '', retain_days: 14, retain_count: 10 })
    updateSchedule.mockReset().mockResolvedValue({})
    deleteBackup.mockReset().mockResolvedValue(undefined)
    listBackups.mockResolvedValue({ items: [] })
    getDownloadURL.mockReset()
    getBackup.mockReset()
    createBackup.mockReset()
    restoreBackup.mockReset()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.restoreAllMocks()
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('keeps a recent backup list when returning to the tab', async () => {
    listBackups.mockResolvedValue({ items: [baseRecord('recent')] })
    const wrapper = mountBackupView(); await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(listBackups).toHaveBeenCalledOnce()
    expect(wrapper.get('tbody').text()).toContain('recent')
  })

  it('silently deduplicates stale return refreshes and preserves unsaved settings', async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2026-11-03T10:00:00Z'))
    listBackups.mockResolvedValue({ items: [baseRecord('existing')] })
    const wrapper = mountBackupView(); await flushPromises()
    const field = wrapper.get('[data-testid="backup-retain-days"]')
    await field.setValue(27)
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    vi.setSystemTime(new Date('2026-11-03T10:02:00Z'))
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('common.loading')
    expect(wrapper.get('tbody').text()).toContain('existing')
    expect((field.element as HTMLInputElement).value).toBe('27')
    finish({ items: [baseRecord('updated')] }); await flushPromises()
    expect(wrapper.get('tbody').text()).toContain('updated')
    expect(getS3Config).toHaveBeenCalledOnce(); expect(getSchedule).toHaveBeenCalledOnce()
  })

  it.each(['backup', 'restore'])('reconciles completed %s work on return and clears busy controls', async (operation) => {
    vi.useFakeTimers()
    listBackups.mockResolvedValueOnce({ items: [{ ...baseRecord('running'),
      status: operation === 'backup' ? 'running' : 'completed',
      restore_status: operation === 'restore' ? 'running' : undefined,
    }] })
    const wrapper = mountBackupView(); await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    listBackups.mockResolvedValue({ items: [baseRecord('running')] })
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('common.loading')
    expect(wrapper.findAll('button').find(b => b.text() === 'admin.backup.operations.createBackup')!.attributes('disabled')).toBeUndefined()
    expect(vi.getTimerCount()).toBe(0)
    expect(getBackup).not.toHaveBeenCalled()
  })

  it.each(['backup', 'restore'])('keeps pending %s commands busy across a return refresh', async (operation) => {
    vi.useFakeTimers()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'prompt').mockReturnValue('synthetic-password')
    listBackups.mockResolvedValue({ items: [baseRecord('existing')] })
    let finish!: (value: object) => void
    const command = operation === 'backup' ? createBackup : restoreBackup
    command.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountBackupView(); await flushPromises()
    const action = () => wrapper.findAll('button').find(b => b.text() === (operation === 'backup'
      ? 'admin.backup.operations.createBackup' : 'admin.backup.actions.restore'))!
    const button = action()
    await button.trigger('click'); await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(2)
    expect(button.attributes('disabled')).toBeDefined()
    let finishList!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finishList = resolve }))
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    finish({ ...baseRecord('existing'), status: operation === 'backup' ? 'running' : 'completed',
      restore_status: operation === 'restore' ? 'running' : undefined })
    await flushPromises()
    finishList({ items: [baseRecord('existing')] }); await flushPromises()
    expect(button.attributes('disabled')).toBeDefined()
    expect(command).toHaveBeenCalledOnce()
    expect(vi.getTimerCount()).toBe(1)
    if (operation === 'backup') expect(wrapper.get('tbody').text()).toContain('admin.backup.status.running')
  })

  it('fetches a fresh list after deletion overlapping a delayed return refresh', async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2026-11-03T10:00:00Z'))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    listBackups.mockResolvedValue({ items: [baseRecord('deleted')] })
    const wrapper = mountBackupView(); await flushPromises()
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    listBackups.mockResolvedValue({ items: [] })
    vi.setSystemTime(new Date('2026-11-03T10:02:00Z'))
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'common.delete')!.trigger('click')
    await flushPromises()
    expect(deleteBackup).toHaveBeenCalledWith('deleted', false)
    finish({ items: [baseRecord('deleted')] }); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(3)
    expect(wrapper.get('tbody').text()).not.toContain('deleted.sql.gz')
    expect(wrapper.findAll('button').some(b => b.text() === 'common.delete')).toBe(false)
  })

  it('retries a required post-delete list invalidated by a failed create command', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    listBackups.mockResolvedValueOnce({ items: [baseRecord('deleted')] })
    const wrapper = mountBackupView(); await flushPromises()
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    listBackups.mockResolvedValue({ items: [] })
    await wrapper.findAll('button').find(b => b.text() === 'common.delete')!.trigger('click')
    await flushPromises()
    createBackup.mockRejectedValueOnce(new Error('synthetic command failure'))
    await wrapper.findAll('button').find(b => b.text() === 'admin.backup.operations.createBackup')!.trigger('click')
    await flushPromises()
    finish({ items: [] }); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(3)
    expect(wrapper.get('tbody').text()).not.toContain('deleted.sql.gz')
  })

  it('retries initial loading invalidated by a failed create command', async () => {
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    listBackups.mockResolvedValue({ items: [baseRecord('existing')] })
    const wrapper = mountBackupView(); await flushPromises()
    createBackup.mockRejectedValueOnce(new Error('synthetic command failure'))
    await wrapper.findAll('button').find(b => b.text() === 'admin.backup.operations.createBackup')!.trigger('click')
    await flushPromises()
    finish({ items: [baseRecord('existing')] }); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(2)
    expect(wrapper.get('tbody').text()).toContain('existing.sql.gz')
  })

  it('resumes paused backup polling after a failed restore invalidates the return list', async () => {
    vi.useFakeTimers()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'prompt').mockReturnValue('synthetic-password')
    const records = [{ ...baseRecord('running'), status: 'running' }, baseRecord('existing')]
    listBackups.mockResolvedValue({ items: records })
    const wrapper = mountBackupView(); await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    expect(vi.getTimerCount()).toBe(0)
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    restoreBackup.mockRejectedValueOnce(new Error('synthetic command failure'))
    await wrapper.findAll('button').find(b => b.text() === 'admin.backup.actions.restore')!.trigger('click')
    await flushPromises()
    finish({ items: records }); await flushPromises()
    expect(listBackups).toHaveBeenCalledTimes(3)
    expect(vi.getTimerCount()).toBe(1)
    getBackup.mockResolvedValue({ ...baseRecord('running'), status: 'running' })
    await vi.advanceTimersByTimeAsync(2000)
    expect(getBackup).toHaveBeenCalledWith('running')
  })

  it.each(['backup', 'restore'])('does not resume %s polling after navigation during initial loading', async (operation) => {
    vi.useFakeTimers()
    let finish!: (value: object) => void
    listBackups.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountBackupView()
    try {
      await flushPromises()
      wrapper.unmount()
      finish({ items: [{ ...baseRecord('pending'),
        status: operation === 'backup' ? 'running' : 'completed',
        restore_status: operation === 'restore' ? 'running' : undefined,
      }] })
      await flushPromises()
      expect(vi.getTimerCount()).toBe(0)
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('显示分卷数并在下载时列出每个分卷链接', async () => {
    listBackups.mockResolvedValue({
      items: [baseRecord('split', [{ index: 1 }, { index: 2 }, { index: 3 }])],
    })
    getDownloadURL.mockResolvedValue({
      parts: [
        { index: 1, size_bytes: 5, url: 'https://example.test/part-1' },
        { index: 2, size_bytes: 6, url: 'https://example.test/part-2' },
        { index: 3, size_bytes: 7, url: 'https://example.test/part-3' },
      ],
    })

    const wrapper = mountBackupView()
    await flushPromises()

    expect(wrapper.text()).toContain('3')
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    expect(downloadButton).toBeDefined()
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:1')
    expect(document.body.textContent).toContain('admin.backup.actions.partLabel:3')
    expect(document.body.querySelector('a[href="https://example.test/part-2"]')).not.toBeNull()
  })

  it('旧单文件记录仍使用单个下载地址', async () => {
    listBackups.mockResolvedValue({ items: [baseRecord('legacy')] })
    getDownloadURL.mockResolvedValue({ url: 'https://example.test/legacy.sql.gz' })

    const wrapper = mountBackupView()
    await flushPromises()
    const downloadButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.backup.actions.download'),
    )
    await downloadButton!.trigger('click')
    await flushPromises()

    expect(getDownloadURL).toHaveBeenCalledWith('legacy')
    expect(document.body.textContent).not.toContain('admin.backup.actions.downloadParts')
  })

  it('运行中的备份不显示删除入口', async () => {
    listBackups.mockResolvedValue({
      items: [{ ...baseRecord('running'), status: 'running', progress: 'uploading' }],
    })

    const wrapper = mountBackupView()
    await flushPromises()

    expect(wrapper.find('tbody tr td:nth-child(5)').text()).toBe('-')
    expect(wrapper.findAll('button').some(button => button.text() === 'common.delete')).toBe(false)
  })

  it('兼容旧配置并保留 0 值，不自动启用月度归档', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 0, retain_count: 0 })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="backup-retain-days"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="backup-retain-count"]').element as HTMLInputElement).value).toBe('0')
    expect((wrapper.get('[data-testid="archive-enabled"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenCalledWith(expect.objectContaining({ retain_days: 0, retain_count: 0, monthly_archive: expect.objectContaining({ enabled: false }) }))
  })

  it('多选日期，手填份数；永久保留隐藏数量并在取消后恢复', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('#backup-archive-options input[value="last"]').setValue(true)
    expect(wrapper.get('#backup-archive-dates').attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    await wrapper.get('[data-testid="archive-count"]').setValue(17)
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'BackupArchiveSettings' }).text()).not.toContain('admin.backup.archive.copies')
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('17')
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 17 } }))
    await wrapper.get('[data-testid="archive-forever"]').setValue(true)
    await wrapper.get('[data-testid="backup-schedule"] .btn-primary').trigger('click')
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: expect.objectContaining({ retain_count: 0 }) }))
  })

  it('空日期和非法份数不可保存，输入 0 不会误选永久保留', async () => {
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('#backup-archive-dates').trigger('click')
    await wrapper.get('#backup-archive-options input[value="1"]').setValue(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('#backup-archive-options input[value="15"]').setValue(true)
    await wrapper.get('[data-testid="archive-forever"]').setValue(false)
    for (const value of ['0', '-1', '1.5', '']) {
      await wrapper.get('[data-testid="archive-count"]').setValue(value)
      expect(save.attributes('disabled')).toBeDefined()
      expect((wrapper.get('[data-testid="archive-forever"]').element as HTMLInputElement).checked).toBe(false)
    }
    await save.trigger('click')
    expect(updateSchedule).not.toHaveBeenCalled()
  })

  it('关闭月度归档后不提交隐藏的归档参数，重新启用时保留编辑值', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 12 } })
    const wrapper = mountBackupView()
    await flushPromises()
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(wrapper.find('[data-testid="archive-count"]').exists()).toBe(false)
    const save = wrapper.get('[data-testid="backup-schedule"] .btn-primary')
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('1')
    // A hidden invalid count neither blocks saving nor reaches the request.
    await wrapper.get('[data-testid="archive-count"]').setValue('')
    expect(save.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="archive-enabled"]').setValue(false)
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: false, days: [1, 15], include_month_end: false, retain_count: 12 } }))
    await wrapper.get('[data-testid="archive-enabled"]').setValue(true)
    await wrapper.get('[data-testid="archive-count"]').setValue(1)
    await save.trigger('click')
    await flushPromises()
    expect(updateSchedule).toHaveBeenLastCalledWith(expect.objectContaining({ monthly_archive: { enabled: true, days: [1, 15], include_month_end: false, retain_count: 1 } }))
  })

  it('加载归档设置并支持键盘关闭日期选择器', async () => {
    getSchedule.mockResolvedValue({ enabled: true, cron_expr: '0 4 * * *', retain_days: 14, retain_count: 10,
      monthly_archive: { enabled: true, days: [1, 15], include_month_end: true, retain_count: 23 } })
    const wrapper = mountBackupView()
    await flushPromises()
    expect((wrapper.get('[data-testid="archive-count"]').element as HTMLInputElement).value).toBe('23')
    await wrapper.get('#backup-archive-dates').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(wrapper.find('#backup-archive-options').exists()).toBe(true)
    expect((wrapper.get('#backup-archive-options input[value="15"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('#backup-archive-options').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('#backup-archive-options').exists()).toBe(false)
  })

  it('归档删除需要单独确认，有限归档不会显示永不过期', async () => {
    listBackups.mockResolvedValue({ items: [{ ...baseRecord('archived'), monthly_archive: { dates: ['2026-09-01', '2026-09-15'], retain_count: 12 } }] })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = mountBackupView()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.backup.archive.badge')
    expect(wrapper.get('tbody tr td:nth-child(6)').text()).toBe('admin.backup.archive.retainLatest')
    const button = wrapper.findAll('button').find(button => button.text() === 'common.delete')!
    await button.trigger('click')
    expect(confirm).toHaveBeenCalledWith('admin.backup.archive.deleteConfirm')
    expect(deleteBackup).not.toHaveBeenCalled()
    confirm.mockReturnValue(true)
    await button.trigger('click')
    await flushPromises()
    expect(deleteBackup).toHaveBeenCalledWith('archived', true)
  })
})
