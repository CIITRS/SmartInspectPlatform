/* eslint-disable */
import { request } from '@umijs/max';

// 定义本地类型
interface User {
  id?: number;
  username?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
  password?: string;
  phone?: string;
  userStatus?: number;
}

interface getUserByNameParams {
  username: string;
}

interface updateUserParams {
  username: string;
}

interface deleteUserParams {
  username: string;
}

interface loginUserParams {
  username: string;
  password: string;
}

/** Create user This can only be done by the logged in user. POST /user */
export async function createUser(body: User, options?: Record<string, unknown>) {
  return request<User>('/user', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** Get user by user name GET /user/${param0} */
export async function getUserByName(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: getUserByNameParams,
  options?: Record<string, unknown>,
) {
  const { username: param0, ...queryParams } = params;
  return request<User>(`/user/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** Updated user This can only be done by the logged in user. PUT /user/${param0} */
export async function updateUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: updateUserParams,
  body: User,
  options?: Record<string, unknown>,
) {
  const { username: param0, ...queryParams } = params;
  return request<User>(`/user/${param0}`, {
    method: 'PUT',
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** Delete user This can only be done by the logged in user. DELETE /user/${param0} */
export async function deleteUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: deleteUserParams,
  options?: Record<string, unknown>,
) {
  const { username: param0, ...queryParams } = params;
  return request<void>(`/user/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** Creates list of users with given input array POST /user/createWithArray */
export async function createUsersWithArrayInput(
  body: User[],
  options?: Record<string, unknown>,
) {
  return request<User[]>('/user/createWithArray', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** Creates list of users with given input array POST /user/createWithList */
export async function createUsersWithListInput(body: User[], options?: Record<string, unknown>) {
  return request<User[]>('/user/createWithList', {
    method: 'POST',
    data: body,
    ...(options || {}),
  });
}

/** Logs user into the system GET /user/login */
export async function loginUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: loginUserParams,
  options?: Record<string, unknown>,
) {
  return request<string>('/user/login', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** Logs out current logged in user session GET /user/logout */
export async function logoutUser(options?: Record<string, unknown>) {
  return request<void>('/user/logout', {
    method: 'GET',
    ...(options || {}),
  });
}
