import { test, expect, gotoApp } from '../../fixtures/test.js'

// What App wires up itself and no component's own test sees.

const URL_FIELD = 'https://api.example.com/endpoint'

test.describe('App behaviour', () => {
  test('pasting a URL with a query moves the parameters out of it', async ({ page }) => {
    await gotoApp(page)
    const url = page.getByPlaceholder(URL_FIELD)
    await url.evaluate((el: HTMLInputElement) => {
      const data = new DataTransfer()
      data.setData('text/plain', 'https://example.com/items?page=2&sort=name')
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }))
    })
    await expect(url).toHaveValue('https://example.com/items')
    await expect(page.getByPlaceholder('Parameter').first()).toHaveValue('page')
  })

  test('pasting a cURL command fills the request', async ({ page }) => {
    await gotoApp(page)
    const url = page.getByPlaceholder(URL_FIELD)
    await url.evaluate((el: HTMLInputElement) => {
      const data = new DataTransfer()
      data.setData('text/plain', "curl -X POST https://example.com/users -d '{}'")
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }))
    })
    await expect(url).toHaveValue('https://example.com/users')
    await expect(page.locator('select').first()).toHaveValue('POST')
  })

  test('the keyboard shortcuts open, switch and close tabs and hide the sidebar', async ({ page }) => {
    await gotoApp(page)
    const tabs = page.getByRole('tab')
    await page.keyboard.press('Control+n')
    await expect(tabs).toHaveCount(2)
    await page.keyboard.press('Control+1')
    await expect(tabs.first()).toHaveAttribute('aria-selected', 'true')
    await page.keyboard.press('Control+Tab')
    await expect(tabs.last()).toHaveAttribute('aria-selected', 'true')
    await page.keyboard.press('Control+w')
    await expect(tabs).toHaveCount(1)

    const sidebar = page.getByTestId('sidebar')
    await expect(sidebar).toBeVisible()
    await page.keyboard.press('Control+b')
    await expect(sidebar).toHaveCount(0)
    await page.keyboard.press('Control+b')
    await expect(sidebar).toBeVisible()
  })

  test('dragging the edge of the sidebar resizes it, and the width is remembered', async ({ page }) => {
    await gotoApp(page)
    const sidebar = page.getByTestId('sidebar')
    const before = (await sidebar.boundingBox())!
    const edge = (await page.locator('[data-testid="sidebar"] + div').boundingBox())!
    await page.mouse.move(edge.x + edge.width / 2, edge.y + 100)
    await page.mouse.down()
    await page.mouse.move(edge.x + 80, edge.y + 100, { steps: 8 })
    await page.mouse.up()
    const after = (await sidebar.boundingBox())!
    expect(after.width).toBeGreaterThan(before.width + 40)
    const stored = await page.evaluate(() => Number(localStorage.getItem('ui.sidebarWidth')))
    expect(stored).toBe(Math.round(after.width))
  })
})
