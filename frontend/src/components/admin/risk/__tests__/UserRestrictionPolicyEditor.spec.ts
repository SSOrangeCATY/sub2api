import { mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import UserRestrictionPolicyEditor from '../UserRestrictionPolicyEditor.vue'
import { cloneUserRestrictionPolicy, validateUserRestrictionPolicy, type UserRestrictionPolicyDraft } from '../userRestrictionPolicy'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => key.endsWith('.rowError') ? key + ': ' + String(params?.row ?? '') + ' ' + String(params?.message ?? '') : key }) }))
const policy = (): UserRestrictionPolicyDraft => ({
  enabled: true, window_days: 7,
  actions: [{ threshold: 2, duration_hours: 24, admin_release: false }, { threshold: 3, duration_hours: 0, admin_release: true }],
})
const props = () => ({ modelValue: policy(), revision: 1, scopeSummary: 'All groups', masterEnabled: true, dirty: true, saving: false })

describe('User restriction policy editor', () => {
  it('shows the confirmed two-stage policy and emits save without changing input', async () => {
    const input = props()
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    expect(wrapper.findAll('.policy-action-row')).toHaveLength(2)
    expect(wrapper.find('[data-test="user-policy-window"]').element).toHaveProperty('value', '7')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('save')).toHaveLength(1)
    expect(input.modelValue).toEqual(policy())
  })
  it('does not mutate the saved policy while editing a threshold', async () => {
    const input = props()
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    await wrapper.find('[data-test="user-policy-threshold-0"]').setValue('4')
    const draft = wrapper.emitted('update:modelValue')?.[0][0] as UserRestrictionPolicyDraft
    expect(draft.actions[0].threshold).toBe(4)
    expect(input.modelValue.actions[0].threshold).toBe(2)
    await wrapper.setProps({ modelValue: draft })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').text()).toContain('threshold')
  })
  it('clears automatic duration when administrator release is selected', async () => {
    const input = props()
    input.modelValue.actions = [{ threshold: 2, duration_hours: 24, admin_release: false }]
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    await wrapper.find('[data-test="user-policy-release-0"]').setValue('manual')
    const draft = wrapper.emitted('update:modelValue')?.[0][0] as UserRestrictionPolicyDraft
    expect(draft.actions[0]).toEqual({ threshold: 2, duration_hours: 0, admin_release: true })
    expect(input.modelValue.actions[0].duration_hours).toBe(24)
  })
  it('keeps blank input invalid and prevents submitting while saving', async () => {
    const wrapper = mount(UserRestrictionPolicyEditor, { props: props() })
    await wrapper.find('[data-test="user-policy-window"]').setValue('')
    const draft = wrapper.emitted('update:modelValue')?.[0][0] as UserRestrictionPolicyDraft
    expect(draft.window_days).toBe('')
    expect(validateUserRestrictionPolicy(draft)).toBe('window')
    await wrapper.setProps({ saving: true })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
  })
  it('preserves independent draft rows and validates all configuration boundaries', () => {
    const saved = policy()
    const draft = cloneUserRestrictionPolicy(saved)
    draft.actions[0].duration_hours = 48
    expect(saved.actions[0].duration_hours).toBe(24)
    expect(validateUserRestrictionPolicy(saved)).toBeNull()
    expect(validateUserRestrictionPolicy({ ...saved, window_days: 31 })).toBe('window')
    expect(validateUserRestrictionPolicy({ ...saved, actions: [] })).toBe('count')
    expect(validateUserRestrictionPolicy({ ...saved, actions: [{ threshold: 2, duration_hours: 0, admin_release: false }] })).toBe('duration')
    expect(validateUserRestrictionPolicy({ ...saved, actions: [{ threshold: 2, duration_hours: 0, admin_release: true }, { threshold: 3, duration_hours: 24, admin_release: false }] })).toBe('manualLast')
  })
})


describe('Reviewed policy-editor boundaries', () => {
  it('does not append after the confirmed terminal rule or silently renumber it', async () => {
    const input = props()
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    const add = wrapper.find('[data-test="user-policy-add"]')
    expect(add.attributes('disabled')).toBeDefined()
    await add.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(input.modelValue.actions.map(action => action.threshold)).toEqual([2, 3])
    expect(wrapper.find('.policy-hint[id]').text()).toContain('terminalHint')
  })
  it('keeps accessible-name and input associations unique for simultaneous editors', () => {
    const Host = defineComponent({
      setup: () => () => h('div', [
        h(UserRestrictionPolicyEditor, { ...props() }),
        h(UserRestrictionPolicyEditor, { ...props() }),
      ]),
    })
    const host = mount(Host)
    const sections = host.findAll('section')
    const headingIds = sections.map(section => section.find('h2').attributes('id'))
    const inputIds = sections.map(section => section.find('[data-test="user-policy-window"]').attributes('id'))
    expect(new Set(headingIds).size).toBe(2)
    expect(new Set(inputIds).size).toBe(2)
    for (const section of sections) {
      expect(section.attributes('aria-labelledby')).toBe(section.find('h2').attributes('id'))
      expect(section.find('label[for]').attributes('for')).toBe(section.find('[data-test="user-policy-window"]').attributes('id'))
    }
  })
  it('marks only the offending threshold and suppresses misleading draft consequences', () => {
    const input = props()
    input.modelValue.actions[1].threshold = 2
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    expect(wrapper.find('[data-test="user-policy-threshold-0"]').attributes('aria-invalid')).not.toBe('true')
    const control = wrapper.find('[data-test="user-policy-threshold-1"]')
    expect(control.attributes('aria-invalid')).toBe('true')
    expect(control.attributes('aria-describedby')).toBe(wrapper.find('[role="alert"]').attributes('id'))
    expect(wrapper.find('.policy-preview').text()).toContain('correctDraft')
    expect(wrapper.find('.policy-preview').text()).not.toContain('previewTemporary')
  })
  it('associates a misplaced terminal-rule error with the release selector', () => {
    const input = props()
    input.modelValue.actions = [
      { threshold: 2, duration_hours: 0, admin_release: true },
      { threshold: 3, duration_hours: 24, admin_release: false },
    ]
    const wrapper = mount(UserRestrictionPolicyEditor, { props: input })
    const control = wrapper.find('[data-test="user-policy-release-0"]')
    expect(control.attributes('aria-invalid')).toBe('true')
    expect(control.attributes('aria-describedby')).toBe(wrapper.find('[role="alert"]').attributes('id'))
    expect(wrapper.find('[data-test="user-policy-release-1"]').attributes('aria-invalid')).not.toBe('true')
  })
  it('labels disabled enforcement separately from a valid draft', () => {
    const wrapper = mount(UserRestrictionPolicyEditor, { props: { ...props(), masterEnabled: false } })
    expect(wrapper.find('.policy-preview').text()).toContain('inactivePreview')
    expect(wrapper.find('[role="status"]').text()).toContain('masterOff')
  })
})

describe('User restriction policy editor reviewed interactions', () => {
  it('explains why an automatic rule at the maximum threshold cannot be extended', async () => {
    const draft = policy()
    draft.actions = [{ threshold: 1000, duration_hours: 24, admin_release: false }]
    const wrapper = mount(UserRestrictionPolicyEditor, { props: { ...props(), modelValue: draft } })
    const button = wrapper.get('[data-test="user-policy-add"]')
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.riskControl.userPolicy.thresholdLimitHint')
    expect(button.attributes('aria-describedby')).toBeTruthy()
    await button.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('preserves previous edits when adding, removing and adding a rule through v-model', async () => {
    const initial = policy()
    initial.actions = [{ threshold: 2, duration_hours: 24, admin_release: false }]
    const Host = defineComponent({
      setup() {
        const draft = ref(initial)
        return () => h(UserRestrictionPolicyEditor, {
          ...props(),
          modelValue: draft.value,
          'onUpdate:modelValue': (value: UserRestrictionPolicyDraft) => { draft.value = value },
        })
      },
    })
    const wrapper = mount(Host)
    await wrapper.get('[data-test="user-policy-duration-0"]').setValue('48')
    await wrapper.get('[data-test="user-policy-add"]').trigger('click')
    expect((wrapper.get('[data-test="user-policy-duration-0"]').element as HTMLInputElement).value).toBe('48')
    expect((wrapper.get('[data-test="user-policy-threshold-1"]').element as HTMLInputElement).value).toBe('3')
    await wrapper.get('[data-test="user-policy-remove-1"]').trigger('click')
    expect(wrapper.find('[data-test="user-policy-threshold-1"]').exists()).toBe(false)
    await wrapper.get('[data-test="user-policy-add"]').trigger('click')
    expect((wrapper.get('[data-test="user-policy-threshold-1"]').element as HTMLInputElement).value).toBe('3')
    expect((wrapper.get('[data-test="user-policy-duration-0"]').element as HTMLInputElement).value).toBe('48')
    expect(initial.actions).toEqual([{ threshold: 2, duration_hours: 24, admin_release: false }])
  })

  it('emits reset only for an editable dirty draft', async () => {
    const wrapper = mount(UserRestrictionPolicyEditor, { props: props() })
    await wrapper.get('[data-test="user-policy-reset"]').trigger('click')
    expect(wrapper.emitted('reset')).toHaveLength(1)
    await wrapper.setProps({ saving: true })
    expect(wrapper.get('[data-test="user-policy-reset"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="user-policy-reset"]').trigger('click')
    expect(wrapper.emitted('reset')).toHaveLength(1)
    await wrapper.setProps({ saving: false, dirty: false })
    expect(wrapper.get('[data-test="user-policy-reset"]').attributes('disabled')).toBeDefined()
  })

  it('prevents submitting an unchanged draft', async () => {
    const wrapper = mount(UserRestrictionPolicyEditor, { props: { ...props(), dirty: false } })
    expect(wrapper.get('[data-test="user-policy-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
  })
})

describe('external policy save blocking', () => {
  it('blocks native form submission during a shared revision conflict', async () => {
    const wrapper = mount(UserRestrictionPolicyEditor, { props: { ...props(), saveBlocked: true } })
    expect(wrapper.get('[data-test="user-policy-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    await wrapper.setProps({ saveBlocked: false })
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toHaveLength(1)
  })
})
