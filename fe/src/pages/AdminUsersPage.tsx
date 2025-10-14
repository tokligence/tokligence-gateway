import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  fetchAdminUsers,
  createAdminUser,
  updateAdminUser,
  setAdminUserStatus,
  deleteAdminUser,
  fetchAdminAPIKeys,
  createAdminAPIKey,
  deleteAdminAPIKey,
} from '../services/api'
import type { AdminUser, AdminAPIKey } from '../types/api'

export function AdminUsersPage() {
  const queryClient = useQueryClient()
  const usersQuery = useQuery({ queryKey: ['admin-users'], queryFn: fetchAdminUsers })
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null)
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState<'gateway_user' | 'gateway_admin'>('gateway_user')
  const [newDisplayName, setNewDisplayName] = useState('')
  const [apiKeyTTL, setApiKeyTTL] = useState('')
  const [apiKeyScopes, setApiKeyScopes] = useState('')
  const [latestToken, setLatestToken] = useState<string | null>(null)

  const createUserMutation = useMutation({
    mutationFn: createAdminUser,
    onSuccess: () => {
      setNewEmail('')
      setNewDisplayName('')
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
    },
  })

  const updateUserMutation = useMutation({
    mutationFn: ({ id, values }: { id: number; values: { role?: string; displayName?: string } }) =>
      updateAdminUser(id, values),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      if (selectedUser && selectedUser.id === variables.id) {
        queryClient.invalidateQueries({ queryKey: ['admin-api-keys', variables.id] })
      }
    },
  })

  const setStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: 'active' | 'inactive' }) => setAdminUserStatus(id, status),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      if (selectedUser && selectedUser.id === variables.id) {
        setSelectedUser(null)
      }
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: deleteAdminUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setSelectedUser(null)
    },
  })

  const apiKeysQuery = useQuery({
    queryKey: ['admin-api-keys', selectedUser?.id],
    queryFn: () => fetchAdminAPIKeys(selectedUser!.id),
    enabled: Boolean(selectedUser?.id),
  })

  const createApiKeyMutation = useMutation({
    mutationFn: ({ userId, scopes, ttl }: { userId: number; scopes: string[]; ttl?: string }) =>
      createAdminAPIKey(userId, { scopes, ttl }),
    onSuccess: ({ token }, variables) => {
      setLatestToken(token)
      queryClient.invalidateQueries({ queryKey: ['admin-api-keys', variables.userId] })
    },
  })

  const deleteApiKeyMutation = useMutation({
    mutationFn: deleteAdminAPIKey,
    onSuccess: () => {
      if (selectedUser) {
        queryClient.invalidateQueries({ queryKey: ['admin-api-keys', selectedUser.id] })
      }
    },
  })

  if (usersQuery.isLoading) {
    return <div className="text-sm text-slate-500">Loading users…</div>
  }

  if (usersQuery.isError) {
    return <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">Failed to load users.</div>
  }

  const users = usersQuery.data ?? []

  return (
    <div className="space-y-8">
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-slate-900">Create user</h2>
        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="text-sm font-medium text-slate-700">
            Email
            <input
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
              value={newEmail}
              onChange={(event) => setNewEmail(event.target.value)}
              placeholder="user@example.com"
            />
          </label>
          <label className="text-sm font-medium text-slate-700">
            Role
            <select
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
              value={newRole}
              onChange={(event) => setNewRole(event.target.value as 'gateway_user' | 'gateway_admin')}
            >
              <option value="gateway_user">Gateway user</option>
              <option value="gateway_admin">Gateway admin</option>
            </select>
          </label>
          <label className="text-sm font-medium text-slate-700">
            Display name
            <input
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
              value={newDisplayName}
              onChange={(event) => setNewDisplayName(event.target.value)}
              placeholder="Optional"
            />
          </label>
        </div>
        <button
          type="button"
          className="mt-4 rounded-lg bg-slate-900 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
          onClick={() =>
            createUserMutation.mutate({ email: newEmail, role: newRole, displayName: newDisplayName })
          }
          disabled={createUserMutation.isPending || !newEmail}
        >
          Create user
        </button>
        {createUserMutation.isError && (
          <p className="mt-2 text-sm text-rose-600">Failed to create user: {(createUserMutation.error as Error).message}</p>
        )}
      </section>

      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-slate-900">Users</h2>
        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-xs uppercase text-slate-500">
                <th className="pb-2">ID</th>
                <th className="pb-2">Email</th>
                <th className="pb-2">Role</th>
                <th className="pb-2">Status</th>
                <th className="pb-2">Display name</th>
                <th className="pb-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-t border-slate-100">
                  <td className="py-2 text-slate-500">{user.id}</td>
                  <td className="py-2">{user.email}</td>
                  <td className="py-2 capitalize">{user.role.replace('gateway_', '')}</td>
                  <td className="py-2">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        user.status === 'active'
                          ? 'bg-emerald-100 text-emerald-700'
                          : 'bg-amber-100 text-amber-700'
                      }`}
                    >
                      {user.status}
                    </span>
                  </td>
                  <td className="py-2 text-slate-500">{user.displayName ?? ''}</td>
                  <td className="py-2 space-x-2">
                    <button
                      className="rounded bg-slate-100 px-2 py-1 text-xs text-slate-700 hover:bg-slate-200"
                      onClick={() => setSelectedUser(user)}
                    >
                      Manage
                    </button>
                    {user.status === 'active' ? (
                      <button
                        className="rounded bg-amber-100 px-2 py-1 text-xs text-amber-700 hover:bg-amber-200"
                        onClick={() => setStatusMutation.mutate({ id: user.id, status: 'inactive' })}
                      >
                        Deactivate
                      </button>
                    ) : (
                      <button
                        className="rounded bg-emerald-100 px-2 py-1 text-xs text-emerald-700 hover:bg-emerald-200"
                        onClick={() => setStatusMutation.mutate({ id: user.id, status: 'active' })}
                      >
                        Activate
                      </button>
                    )}
                    {user.role !== 'root_admin' && (
                      <button
                        className="rounded bg-rose-100 px-2 py-1 text-xs text-rose-700 hover:bg-rose-200"
                        onClick={() => deleteUserMutation.mutate(user.id)}
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {selectedUser && (
        <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
          <header className="flex items-center justify-between">
            <div>
              <h3 className="text-base font-semibold text-slate-900">{selectedUser.email}</h3>
              <p className="text-xs text-slate-500">Update role or generate API keys</p>
            </div>
            <button
              className="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-600 hover:bg-slate-200"
              onClick={() => setSelectedUser(null)}
            >
              Close
            </button>
          </header>

          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <label className="text-sm font-medium text-slate-700">
              Role
              <select
                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                value={selectedUser.role}
                onChange={(event) =>
                  updateUserMutation.mutate({
                    id: selectedUser.id,
                    values: { role: event.target.value },
                  })
                }
              >
                <option value="gateway_user">Gateway user</option>
                <option value="gateway_admin">Gateway admin</option>
              </select>
            </label>
            <label className="text-sm font-medium text-slate-700">
              Display name
              <input
                className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                defaultValue={selectedUser.displayName ?? ''}
                onBlur={(event) =>
                  updateUserMutation.mutate({
                    id: selectedUser.id,
                    values: { displayName: event.target.value },
                  })
                }
              />
            </label>
          </div>

          <div className="mt-6 space-y-4">
            <div>
              <h4 className="text-sm font-semibold text-slate-900">Issue API key</h4>
              <div className="mt-2 grid gap-3 md:grid-cols-3">
                <input
                  className="rounded-lg border border-slate-300 px-3 py-2"
                  placeholder="Scopes (comma separated)"
                  value={apiKeyScopes}
                  onChange={(event) => setApiKeyScopes(event.target.value)}
                />
                <input
                  className="rounded-lg border border-slate-300 px-3 py-2"
                  placeholder="TTL in hours (optional)"
                  value={apiKeyTTL}
                  onChange={(event) => setApiKeyTTL(event.target.value)}
                />
                <button
                  className="rounded-lg bg-slate-900 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
                  onClick={() =>
                    createApiKeyMutation.mutate({
                      userId: selectedUser.id,
                      scopes: apiKeyScopes ? apiKeyScopes.split(',').map((s) => s.trim()).filter(Boolean) : [],
                      ttl: apiKeyTTL,
                    })
                  }
                  disabled={createApiKeyMutation.isPending}
                >
                  Create API key
                </button>
              </div>
              {latestToken && (
                <p className="mt-2 text-xs text-emerald-700">Token (copy now): {latestToken}</p>
              )}
            </div>

            <div>
              <h4 className="text-sm font-semibold text-slate-900">Existing API keys</h4>
              {apiKeysQuery.isLoading && <p className="mt-2 text-xs text-slate-500">Loading…</p>}
              {apiKeysQuery.isSuccess && apiKeysQuery.data.length === 0 && (
                <p className="mt-2 text-xs text-slate-500">No keys issued yet.</p>
              )}
              {apiKeysQuery.isSuccess && apiKeysQuery.data.length > 0 && (
                <ul className="mt-2 space-y-2 text-xs text-slate-600">
                  {apiKeysQuery.data.map((key: AdminAPIKey) => (
                    <li key={key.id} className="flex items-center justify-between rounded border border-slate-200 px-3 py-2">
                      <div>
                        <p className="font-medium text-slate-800">{key.prefix}</p>
                        <p className="text-slate-500">
                          {key.scopes.length > 0 ? key.scopes.join(', ') : 'default scopes'} |
                          {key.expiresAt ? ` expires ${new Date(key.expiresAt).toLocaleString()}` : ' no expiry'}
                        </p>
                      </div>
                      <button
                        className="rounded bg-rose-100 px-2 py-1 text-rose-700 hover:bg-rose-200"
                        onClick={() => deleteApiKeyMutation.mutate(key.id)}
                      >
                        Revoke
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </section>
      )}
    </div>
  )
}
