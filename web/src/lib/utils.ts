import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

/**
 * 样式类名合并工具函数（shadcn/ui 规范）
 * 支持条件判断与 Tailwind 类名自动消歧合并
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
