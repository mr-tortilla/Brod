#!/bin/sh
# Права на управление сетью для режима TUN: без них пришлось бы запускать приложение от root.
# Права слетают при замене бинарника, поэтому выдаются заново при каждой установке и обновлении.
if command -v setcap >/dev/null 2>&1; then
	setcap cap_net_admin,cap_net_raw,cap_net_bind_service+ep /usr/bin/brod ||
		echo "brod: не удалось выдать права для режима TUN, режим «Прокси» будет работать" >&2
else
	echo "brod: setcap не найден, режим TUN недоступен, режим «Прокси» будет работать" >&2
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi
exit 0
