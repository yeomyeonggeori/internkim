case $- in
  *i*)
    if [ -t 1 ] && command -v fastfetch >/dev/null; then
      if [ "$TERM" = linux ]; then
        fastfetch --logo /etc/xdg/fastfetch/kim-mini-logo-console
      else
        fastfetch
      fi
    fi
    ;;
esac
