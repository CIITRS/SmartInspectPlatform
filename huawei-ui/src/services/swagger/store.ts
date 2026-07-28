/* eslint-disable */
import { request } from '@umijs/max';

// 定义本地类型
interface Order {
  id?: number;
  petId?: number;
  quantity?: number;
  shipDate?: string;
  status?: 'placed' | 'approved' | 'delivered';
  complete?: boolean;
}

interface getOrderByIdParams {
  orderId: number;
}

interface deleteOrderParams {
  orderId: number;
}

/** Returns pet inventories by status Returns a map of status codes to quantities GET /store/inventory */
export async function getInventory(options?: { [key: string]: unknown }) {
  return request<Record<string, unknown>>('/store/inventory', {
    method: 'GET',
    ...(options || {}),
  });
}

/** Place an order for a pet POST /store/order */
export async function placeOrder(body: Order, options?: { [key: string]: unknown }) {
  return request<Order>('/store/order', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** Find purchase order by ID For valid response try integer IDs with value >= 1 and <= 10.         Other values will generated exceptions GET /store/order/${param0} */
export async function getOrderById(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: getOrderByIdParams,
  options?: { [key: string]: unknown },
) {
  const { orderId: param0, ...queryParams } = params;
  return request<Order>(`/store/order/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** Delete purchase order by ID For valid response try integer IDs with positive integer value.         Negative or non-integer values will generate API errors DELETE /store/order/${param0} */
export async function deleteOrder(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: deleteOrderParams,
  options?: { [key: string]: unknown },
) {
  const { orderId: param0, ...queryParams } = params;
  return request<void>(`/store/order/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
