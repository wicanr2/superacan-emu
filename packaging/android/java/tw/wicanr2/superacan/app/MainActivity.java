package tw.wicanr2.superacan.app;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Toast;

import tw.wicanr2.superacan.acan.Acan;
import tw.wicanr2.superacan.acan.EbitenView;

/**
 * Activity 只做四件事：啟動、離開前景、回到前景、返回鍵。所有其他行為都在 Go 端，
 * 契約寫在 docs/android-frontend.md。
 */
public class MainActivity extends Activity {
    private EbitenView view;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        try {
            // getExternalFilesDir(null) 不需要任何權限，使用者又能用檔案管理程式
            // 或 USB 把韌體與卡帶放進去。
            Acan.start(getExternalFilesDir(null).getAbsolutePath());
        } catch (Exception e) {
            Toast.makeText(this, e.getMessage(), Toast.LENGTH_LONG).show();
            finish();
            return;
        }
        view = new EbitenView(this);
        setContentView(view);
    }

    @Override
    protected void onPause() {
        // 行動平台沒有正常結束：切走之後程式可能直接被回收，所以這一刻是最後一次
        // 能寫檔的機會。先讓 Go 端落地，再暫停畫面。
        Acan.suspend();
        if (view != null) {
            view.suspendGame();
        }
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (view != null) {
            view.resumeGame();
        }
        Acan.resume();
    }

    @Override
    public void onBackPressed() {
        // 介面一律吃掉返回鍵：遊戲中開選單、選單中退一層，所以返回鍵不會把應用
        // 程式關掉。要離開請用選單裡的「離開模擬器」。
        if (!Acan.back()) {
            super.onBackPressed();
        }
    }
}
