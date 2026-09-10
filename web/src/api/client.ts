const BASE = '/api'

// api 统一请求：非零 code 抛错
export async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  let j: { code: number; data: T; msg: string }
  try {
    j = await r.json()
  } catch {
    throw new Error('服务器响应异常（HTTP ' + r.status + '）')
  }
  if (j.code !== 0) throw new Error(j.msg || '请求失败')
  return j.data
}
